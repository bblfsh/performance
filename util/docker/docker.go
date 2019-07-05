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

const (
	// TODO(lwsanty): maybe we need to run container in stateless mode and install driver version that we want
	// bblfshd default configuration
	bblfshdImage     = "bblfsh/bblfshd"
	bblfshdContainer = "bblfshd-perf"
	bblfshdPort      = "9432"
)

var (
	errRunFailed             = errors.NewKind("cannot run bblfshd container")
	errConnectToDockerFailed = errors.NewKind("could not connect to docker")
	errResourceStartFailed   = errors.NewKind("could not start resource")
	errPortWaitTimeout       = errors.NewKind("could not wait until port %d is enabled")
)

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
			PortBindings: map[docker.Port][]docker.PortBinding{
				bblfshdPort: {{HostPort: bblfshdPort}},
			},
		})
	if err != nil {
		return "", nil, wrapErr(err, errResourceStartFailed)
	}

	addr := resource.GetHostPort(bblfshdPort + "/tcp")
	log.Debugf("addr used: %s\n", addr)
	if err := pool.Retry(func() error {
		conn, err := net.DialTimeout("tcp", addr, time.Second/4)
		if err == nil {
			conn.Close()
			return nil
		}
		return nil
	}); err != nil {
		return "", nil, wrapErr(errPortWaitTimeout.New(bblfshdPort))
	}

	return addr, func() {
		if err := pool.Purge(resource); err != nil {
			log.Errorf(err, "could not purge resource: %s", resource.Container.Name)
		}
	}, nil
}

func wrapErr(err error, kinds ...*errors.Kind) error {
	return errRunFailed.Wrap(util.WrapErr(err, kinds...))
}
