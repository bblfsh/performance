package docker

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/bblfsh/performance"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

const (
	// TODO(lwsanty): maybe we need to run container in the stateless mode and install driver version that we want
	// bblfshd default configuration
	bblfshdImage     = "bblfsh/bblfshd"
	bblfshdContainer = "bblfshd-perf"
	bblfshdPort      = "9432"

	// driver default configuration
	driverContainer = "driver"
)

var (
	errRunContainerFailed    = errors.NewKind("cannot run container")
	errConnectToDockerFailed = errors.NewKind("could not connect to docker")
	errResourceStartFailed   = errors.NewKind("could not start resource")
	errPortWaitTimeout       = errors.NewKind("could not wait until port %v is enabled")
	errExecFailed            = errors.NewKind("failed to exec command")
	errUploadFailed          = errors.NewKind("upload failed")
	errGetResultsFailed      = errors.NewKind("get results failed")
)

// Driver is a struct that eases interaction with driver container
type Driver struct {
	// Address is a driver's GRPC address
	Address string
	// Pool contains docker client for further interaction with container
	Pool *dockertest.Pool
	// Resource contains container metadata
	Resource *dockertest.Resource
}

// TODO(lwsanty): use UploadToContainer for more various data
// Upload uploads given file from host to container
// source file's descriptor is used as input stream
// docker executes cat command to redirect this stream to the destination file
func (d *Driver) Upload(ctx context.Context, src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return errUploadFailed.Wrap(err)
	}
	defer f.Close()

	sh := fmt.Sprintf("cat > %[1]s ; chmod +x %[1]s", dst)
	exec, err := d.Pool.Client.CreateExec(docker.CreateExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Context:      ctx,
		Cmd:          []string{"sh", "-c", sh},
		Container:    d.Resource.Container.ID,
		Privileged:   true,
	})
	if err != nil {
		return errUploadFailed.Wrap(err)
	}

	err = d.Pool.Client.StartExec(exec.ID, docker.StartExecOptions{
		Context:      ctx,
		InputStream:  f,
		OutputStream: os.Stderr,
		ErrorStream:  os.Stderr,
	})
	if err != nil {
		return errUploadFailed.Wrap(err)
	}

	return nil
}

// GetResults reads result files inside the container
// Do not use it on large files!
func (d *Driver) GetResults(ctx context.Context, src string) ([]byte, error) {
	exec, err := d.Pool.Client.CreateExec(docker.CreateExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Context:      ctx,
		Cmd:          []string{"sh", "-c", "cat " + src},
		Container:    d.Resource.Container.ID,
		Privileged:   true,
	})
	if err != nil {
		return nil, errGetResultsFailed.Wrap(err)
	}

	var buf bytes.Buffer
	err = d.Pool.Client.StartExec(exec.ID, docker.StartExecOptions{
		Context:      ctx,
		OutputStream: &buf,
		ErrorStream:  os.Stderr,
	})
	if err != nil {
		return nil, errGetResultsFailed.Wrap(err)
	}

	return buf.Bytes(), nil
}

// Exec executes given command inside the driver, sends output to Stdout and returns error if commands exit code != 0
func (d *Driver) Exec(ctx context.Context, envs []string, cmd ...string) error {
	log.Debugf("creating exec")
	exec, err := d.Pool.Client.CreateExec(docker.CreateExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Env:          envs,
		Context:      ctx,
		Cmd:          cmd,
		Container:    d.Resource.Container.ID,
		Privileged:   true,
	})
	if err != nil {
		return errExecFailed.Wrap(err)
	}

	log.Debugf("starting exec")
	err = d.Pool.Client.StartExec(exec.ID, docker.StartExecOptions{
		Context:      ctx,
		RawTerminal:  true,
		Tty:          true,
		OutputStream: os.Stderr,
		ErrorStream:  os.Stderr,
	})
	if err != nil {
		return errExecFailed.Wrap(err)
	}

	inspect, err := d.Pool.Client.InspectExec(exec.ID)
	if err != nil {
		return errExecFailed.Wrap(err)
	}

	log.Debugf("container: %v\ncode:%v\n%v\n%+v\n", inspect.ContainerID, inspect.ExitCode, inspect.Running, inspect.ProcessConfig)
	if inspect.ExitCode != 0 {
		return errExecFailed.Wrap(fmt.Errorf("code: %v", inspect.ExitCode))
	}

	return nil
}

// Close removes container
func (d *Driver) Close() { purge(d.Pool, d.Resource) }

// RunBblfshd pulls and runs bblfshd container with a given tag, waits until the port is ready and returns
// endpoint address and a closer that performs post-cleanup
func RunBblfshd(tag string) (string, func(), error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", nil, wrapErr(err, errConnectToDockerFailed)
	}

	resource, err := pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:         bblfshdContainer,
			Repository:   bblfshdImage,
			Tag:          tag,
			Privileged:   true,
			ExposedPorts: []string{bblfshdPort},
			Mounts:       []string{"/var/run/docker.sock:/var/run/docker.sock"},
			PortBindings: map[docker.Port][]docker.PortBinding{
				bblfshdPort: {{HostPort: bblfshdPort}},
			},
		})
	if err != nil {
		return "", nil, wrapErr(err, errResourceStartFailed)
	}

	addr := resource.GetHostPort(bblfshdPort + "/tcp")
	log.Debugf("addr used: %s", addr)
	if err := wait(pool, addr); err != nil {
		purge(pool, resource)
		return "", nil, err
	}

	return addr, func() { purge(pool, resource) }, nil
}

// RunDriver runs driver of given image and mounts
func RunDriver(image *Image, mounts ...string) (*Driver, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, wrapErr(err, errConnectToDockerFailed)
	}

	resource, err := pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:         driverContainer,
			Repository:   image.toString(false),
			Tag:          image.Tag,
			Privileged:   true,
			ExposedPorts: []string{bblfshdPort},
			Mounts:       append(mounts, "/var/run/docker.sock:/var/run/docker.sock"),
			PortBindings: map[docker.Port][]docker.PortBinding{
				bblfshdPort: {{HostPort: bblfshdPort}},
			},
		})
	if err != nil {
		return nil, wrapErr(err, errResourceStartFailed)
	}

	addr := resource.GetHostPort(bblfshdPort + "/tcp")
	log.Debugf("addr used: %s", addr)
	log.Debugf("waiting for port")
	if err := wait(pool, addr); err != nil {
		purge(pool, resource)
		return nil, err
	}

	return &Driver{
		Address:  addr,
		Pool:     pool,
		Resource: resource,
	}, nil
}

func wait(pool *dockertest.Pool, addr string) error {
	return pool.Retry(func() error {
		conn, err := net.DialTimeout("tcp", addr, time.Second/4)
		if err == nil {
			conn.Close()
			return nil
		}
		return wrapErr(errPortWaitTimeout.New(bblfshdPort))
	})
}

func purge(p *dockertest.Pool, resources ...*dockertest.Resource) {
	for _, r := range resources {
		if err := p.Purge(r); err != nil {
			log.Errorf(err, "could not purge resource: %s", r.Container.Name)
		}
	}
}

func wrapErr(err error, kinds ...*errors.Kind) error {
	return errRunContainerFailed.Wrap(performance.WrapErr(err, kinds...))
}
