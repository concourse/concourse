package integration_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/containerd/containerd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tedsuo/ifrit"
)

type IntegrationSuite struct {
	suite.Suite
	*require.Assertions

	backend           runtime.Backend
	client            *libcontainerd.Client
	containerdProcess ifrit.Process
	rootfs            string
	stderr            bytes.Buffer
	stdout            bytes.Buffer
	tmpDir            string
}

func (s *IntegrationSuite) containerdSocket() string {
	return filepath.Join(s.tmpDir, "containerd.sock")
}

func (s *IntegrationSuite) startContainerd() {
	command := exec.Command("/usr/local/concourse/bin/containerd",
		"--address="+s.containerdSocket(),
		"--root="+filepath.Join(s.tmpDir, "containerd"),
	)

	command.Stdout = &s.stdout
	command.Stderr = &s.stderr
	command.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	s.containerdProcess = ifrit.Invoke(workercmd.CmdRunner{command})
}

func (s *IntegrationSuite) SetupSuite() {
	var err error
	s.tmpDir, err = ioutil.TempDir("", "containerd")
	s.NoError(err)

	s.startContainerd()

	retries := 0
	for retries < 10 {
		c, err := containerd.New(s.containerdSocket(), containerd.WithTimeout(100*time.Millisecond))
		if err != nil {
			retries++
			continue
		}

		c.Close()
		return
	}

	fmt.Println("STDOUT:", s.stdout.String())
	fmt.Println("STDERR:", s.stderr.String())
	s.Fail("timed out waiting for containerd to start")
}

func (s *IntegrationSuite) TearDownSuite() {
	s.containerdProcess.Signal(syscall.SIGTERM)
	<-s.containerdProcess.Wait()

	os.RemoveAll(s.tmpDir)
}

func (s *IntegrationSuite) SetupTest() {
	var (
		err            error
		namespace      = "test"
		requestTimeout = 3 * time.Second
	)

	s.backend, err = runtime.New(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
	)
	s.NoError(err)
	s.NoError(s.backend.Start())

	s.setupRootfs()
}

func (s *IntegrationSuite) setupRootfs() {
	var err error

	s.rootfs, err = ioutil.TempDir("", "containerd-integration")
	s.NoError(err)

	cmd := exec.Command("go", "build",
		"-tags", "netgo",
		"-o", filepath.Join(s.rootfs, "executable"),
		"./sample/main.go",
	)

	err = cmd.Run()
	s.NoError(err)

	return
}

func (s *IntegrationSuite) TearDownTest() {
	s.backend.Stop()
	os.RemoveAll(s.rootfs)
}

func (s *IntegrationSuite) TestPing() {
	s.NoError(s.backend.Ping())
}

// TestContainerCreateRunStopDestroy validates that we're able to go through the
// whole lifecycle of:
//
// 1. creating the container
// 2. running a process in it
// 3. stopping the process
// 4. deleting the container
//
func (s *IntegrationSuite) TestContainerCreateRunStopedDestroy() {
	handle := uuid()
	properties := garden.Properties{"test": uuid()}

	_, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		Properties: properties,
	})
	s.NoError(err)

	containers, err := s.backend.Containers(properties)
	s.NoError(err)

	s.Len(containers, 1)

	err = s.backend.Destroy(handle)
	s.NoError(err)

	containers, err = s.backend.Containers(properties)
	s.NoError(err)
	s.Len(containers, 0)
}

// TestContainerNetworkEgress aims at verifying that a process that we run in a
// container that we create through our backend is able to make requests to
// external services.
//
func (s *IntegrationSuite) TestContainerNetworkEgress() {
	handle := uuid()

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.backend.Destroy(handle))
	}()

	buf := new(buffer)
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{
				"-http-get=http://example.com",
			},
		},
		garden.ProcessIO{
			Stdout: buf,
			Stderr: buf,
		},
	)
	s.NoError(err)

	exitCode, err := proc.Wait()
	s.NoError(err)

	s.Equal(exitCode, 0)
	s.Equal("200 OK\n", buf.String())
}

// TestRunPrivileged tests whether we're able to run a process in a privileged
// container.
//
func (s *IntegrationSuite) TestRunPrivileged() {
	s.runToCompletion(true)
}

// TestRunPrivileged tests whether we're able to run a process in an
// unprivileged container.
//
// Differently from the privileged counterpart, we first need to change the
// ownership of the rootfs so the uid 0 inside the container has the permissions
// to execute the executable in there.
//
func (s *IntegrationSuite) TestRunUnprivileged() {
	maxUid, maxGid, err := runtime.NewUserNamespace().MaxValidIds()
	s.NoError(err)

	filepath.Walk(s.rootfs, func(path string, _ os.FileInfo, _ error) error {
		return os.Lchown(path, int(maxUid), int(maxGid))
	})

	s.runToCompletion(false)
}

func (s *IntegrationSuite) runToCompletion(privileged bool) {
	handle := uuid()

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: privileged,
		Env: []string{
			"FOO=bar",
		},
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.backend.Destroy(handle))
	}()

	buf := new(buffer)
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Dir:  "/somewhere",
		},
		garden.ProcessIO{
			Stdout: buf,
			Stderr: buf,
		},
	)
	s.NoError(err)

	exitCode, err := proc.Wait()
	s.NoError(err)

	s.Equal(exitCode, 0)
	s.Equal("hello world\n", buf.String())

}

// TestAttachToUnknownProc verifies that trying to attach to a process that does
// not exist lead to an error.
//
func (s *IntegrationSuite) TestAttachToUnknownProc() {
	handle := uuid()

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.backend.Destroy(handle))
	}()

	_, err = container.Attach("inexistent", garden.ProcessIO{
		Stdout: ioutil.Discard,
		Stderr: ioutil.Discard,
	})
	s.Error(err)
}

// TestAttach tries to validate that we're able to start a process in a
// container, get rid of the original client that originated the process, and
// then attach back to that process from a new client.
//
func (s *IntegrationSuite) TestAttach() {
	handle := uuid()

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	lockedBuffer := new(buffer)
	lockedBuffer.Lock()

	originalProc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{
				"-write-many-times=aa",
			},
		},
		garden.ProcessIO{
			Stdout: lockedBuffer,
			Stderr: lockedBuffer,
		},
	)
	s.NoError(err)

	id := originalProc.ID()

	// kill the conn, and attach

	s.backend.Stop()
	s.NoError(s.backend.Start())

	container, err = s.backend.Lookup(handle)
	s.NoError(err)

	buf := new(buffer)
	proc, err := container.Attach(id, garden.ProcessIO{
		Stdout: buf,
		Stderr: buf,
	})
	s.NoError(err)

	exitCode, err := proc.Wait()
	s.NoError(err)

	s.Equal(exitCode, 0)
	s.Contains(buf.String(), "aa\naa\naa\naa\naa\naa\n")

	err = s.backend.Destroy(container.Handle())
	s.NoError(err)
}

// TestCustomDNS verfies that when a network is setup with custom NameServers
// those NameServers should appear in the container's etc/resolv.conf
//
func (s *IntegrationSuite) TestCustomDNS() {
	namespace := "test-custom-dns"
	requestTimeout := 3 * time.Second

	network, err := runtime.NewCNINetwork(
		runtime.WithNameServers([]string{
			"1.1.1.1", "1.2.3.4",
		}),
	)
	s.NoError(err)

	networkOpt := runtime.WithNetwork(network)
	customBackend, err := runtime.New(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		networkOpt,
	)
	s.NoError(err)

	s.NoError(customBackend.Start())

	handle := uuid()

	container, err := customBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(customBackend.Destroy(handle))
		customBackend.Stop()
	}()

	buf := new(buffer)

	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{
				"-cat",
				"/etc/resolv.conf",
			},
		},
		garden.ProcessIO{
			Stdout: buf,
			Stderr: buf,
		},
	)
	s.NoError(err)

	exitCode, err := proc.Wait()
	s.NoError(err)

	s.Equal(exitCode, 0)
	expectedDNSServer := "nameserver 1.1.1.1\nnameserver 1.2.3.4\n"
	s.Equal(expectedDNSServer, buf.String())
}

// TestUngracefulStop aims at validating that we're giving the process enough
// opportunity to finish, but that at the same time, we don't wait forever.
//
func (s *IntegrationSuite) TestUngracefulStop() {
	var ungraceful = true
	s.testStop(ungraceful)
}

// TestGracefulStop aims at validating that we're giving the process enough
// opportunity to finish, but that at the same time, we don't wait forever.
//
func (s *IntegrationSuite) TestGracefulStop() {
	var ungraceful = false
	s.testStop(ungraceful)
}

func (s *IntegrationSuite) testStop(kill bool) {
	handle := uuid()
	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.backend.Destroy(handle))
	}()

	_, err = container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{"-wait-for-signal=sighup"},
		},
		garden.ProcessIO{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		},
	)
	s.NoError(err)
	s.NoError(container.Stop(kill))
}
