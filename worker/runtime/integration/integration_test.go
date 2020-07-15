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
	"github.com/containerd/containerd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

//Note: Some of these integration tests call on functionality that manipulates
//the iptable rule set. They lack isolation and, therefore, should never be run in parallel.
type IntegrationSuite struct {
	suite.Suite
	*require.Assertions

	gardenBackend     runtime.GardenBackend
	client            *libcontainerd.Client
	containerdProcess *exec.Cmd
	rootfs            string
	stderr            bytes.Buffer
	stdout            bytes.Buffer
	tmpDir            string
}

func (s *IntegrationSuite) containerdSocket() string {
	return filepath.Join(s.tmpDir, "containerd.sock")
}

func (s *IntegrationSuite) startContainerd() {
	command := exec.Command("containerd",
		"--address="+s.containerdSocket(),
		"--root="+filepath.Join(s.tmpDir, "root"),
		"--state="+filepath.Join(s.tmpDir, "state"),
	)

	command.Stdout = &s.stdout
	command.Stderr = &s.stderr
	command.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	err := command.Start()
	s.NoError(err)

	s.containerdProcess = command
}

func (s *IntegrationSuite) stopContainerd() {
	s.NoError(s.containerdProcess.Process.Signal(syscall.SIGTERM))
	s.NoError(s.containerdProcess.Wait())
}

func (s *IntegrationSuite) SetupSuite() {
	var err error
	s.tmpDir, err = ioutil.TempDir("", "containerd")
	s.NoError(err)

	s.startContainerd()

	retries := 0
	for retries < 100 {
		c, err := containerd.New(s.containerdSocket(), containerd.WithTimeout(100*time.Millisecond))
		if err != nil {
			retries++
			continue
		}

		c.Close()
		return
	}

	s.stopContainerd()
	s.NoError(os.RemoveAll(s.tmpDir))

	fmt.Println("STDOUT:", s.stdout.String())
	fmt.Println("STDERR:", s.stderr.String())
	s.Fail("timed out waiting for containerd to start")
}

func (s *IntegrationSuite) TearDownSuite() {
	s.stopContainerd()
	s.NoError(os.RemoveAll(s.tmpDir))
}

func (s *IntegrationSuite) SetupTest() {
	var (
		err            error
		namespace      = "test"
		requestTimeout = 3 * time.Second
	)

	s.gardenBackend, err = runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
	)
	s.NoError(err)
	s.NoError(s.gardenBackend.Start())

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
	s.gardenBackend.Stop()
	os.RemoveAll(s.rootfs)
	s.cleanupIptables()
}

func (s *IntegrationSuite) cleanupIptables() {
	//Flush all rules
	exec.Command("iptables", "-F").Run()
	//Delete all user-defined chains
	exec.Command("iptables", "-X").Run()
}

func (s *IntegrationSuite) TestPing() {
	s.NoError(s.gardenBackend.Ping())
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

	_, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		Properties: properties,
	})
	s.NoError(err)

	containers, err := s.gardenBackend.Containers(properties)
	s.NoError(err)

	s.Len(containers, 1)

	err = s.gardenBackend.Destroy(handle)
	s.NoError(err)

	containers, err = s.gardenBackend.Containers(properties)
	s.NoError(err)
	s.Len(containers, 0)
}

// TestContainerNetworkEgress aims at verifying that a process that we run in a
// container that we create through our gardenBackend is able to make requests to
// external services.
//
func (s *IntegrationSuite) TestContainerNetworkEgress() {
	handle := uuid()

	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.gardenBackend.Destroy(handle))
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

// TestContainerNetworkEgressWithRestrictedNetworks verifies that a process that we run in a
// container that we create through our gardenBackend is not able to reach an address that
// we have blocked access to.
//
func (s *IntegrationSuite) TestContainerNetworkEgressWithRestrictedNetworks() {
	namespace := "test-restricted-networks"
	requestTimeout := 3 * time.Second

	network, err := runtime.NewCNINetwork(
		runtime.WithRestrictedNetworks([]string{"1.1.1.1"}),
	)

	s.NoError(err)

	networkOpt := runtime.WithNetwork(network)
	customBackend, err := runtime.NewGardenBackend(
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
				"-http-get=http://1.1.1.1",
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

	s.Equal(exitCode, 1, "Process in container should not be able to connect to restricted network")
	s.Contains(buf.String(), "connect: connection refused")
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

	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: privileged,
		Env: []string{
			"FOO=bar",
		},
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.gardenBackend.Destroy(handle))
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

	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.gardenBackend.Destroy(handle))
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

	container, err := s.gardenBackend.Create(garden.ContainerSpec{
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

	s.gardenBackend.Stop()
	s.NoError(s.gardenBackend.Start())

	container, err = s.gardenBackend.Lookup(handle)
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

	err = s.gardenBackend.Destroy(container.Handle())
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
	customBackend, err := runtime.NewGardenBackend(
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
	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.gardenBackend.Destroy(handle))
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
