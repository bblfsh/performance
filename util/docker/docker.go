package docker

import (
	"net"
	"time"

	"github.com/bblfsh/performance/util"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

// TODO maybe we need to run container in stateless mode and install driver version that we want
const (
	image = "bblfsh/bblfshd"
	tag   = "latest-drivers"

	containerName = "bblfshd-perf"
	port          = "9432"
)

var (
	errRunFailed             = errors.NewKind("cannot run bblfshd container")
	errConnectToDockerFailed = errors.NewKind("could not connect to docker")
	errResourceStartFailed   = errors.NewKind("could not start resource")
	errPortWaitTimeout       = errors.NewKind("could not wait until port %d is enabled")
)

func Run() (string, func(), error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", nil, wrapErr(err, errConnectToDockerFailed)
	}

	resource, err := pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:         containerName,
			Repository:   image,
			Tag:          tag,
			Privileged:   true,
			ExposedPorts: []string{port},
			PortBindings: map[docker.Port][]docker.PortBinding{
				port: {{HostPort: port}},
			},
		})
	if err != nil {
		return "", nil, wrapErr(err, errResourceStartFailed)
	}

	ip := resource.GetBoundIP(resource.Container.ID)
	if err := pool.Retry(func() error {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), time.Second/4)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		return nil
	}); err != nil {
		return "", nil, wrapErr(errPortWaitTimeout.New(port))
	}

	return "localhost:" + port, func() {
		if err := pool.Purge(resource); err != nil {
			log.Errorf(err, "could not purge resource: %s", resource.Container.Name)
		}
	}, nil
}

func wrapErr(err error, kinds ...*errors.Kind) error {
	return errRunFailed.Wrap(util.WrapErr(err, kinds...))
}
