package integration_test

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing/iotest"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/containerd/containerd"
	"github.com/jackpal/gateway"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Note: Some of these integration tests call on functionality that manipulates
// the iptable rule set. They lack isolation and, therefore, should never be run in parallel.
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
	configPath := filepath.Join(s.tmpDir, "containerd.toml")
	err := workercmd.WriteDefaultContainerdConfig(configPath)
	s.NoError(err)

	command := exec.Command("containerd",
		"--address="+s.containerdSocket(),
		"--root="+filepath.Join(s.tmpDir, "root"),
		"--state="+filepath.Join(s.tmpDir, "state"),
		"--config="+configPath,
	)

	command.Stdout = &s.stdout
	command.Stderr = &s.stderr
	command.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	err = command.Start()
	s.NoError(err)

	s.containerdProcess = command
}

func (s *IntegrationSuite) stopContainerd() {
	s.NoError(s.containerdProcess.Process.Signal(syscall.SIGTERM))
	s.NoError(s.containerdProcess.Wait())
}

func (s *IntegrationSuite) SetupSuite() {
	var err error
	s.tmpDir, err = os.MkdirTemp("", "containerd")
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

func (s *IntegrationSuite) BeforeTest(suiteName, testName string) {
	var (
		err            error
		namespace      = "test"
		requestTimeout = 3 * time.Second
	)

	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
	)

	s.NoError(err)

	s.gardenBackend, err = runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(requestTimeout),
	)
	s.NoError(err)
	s.NoError(s.gardenBackend.Start())

	s.setupRootfs()
}

func (s *IntegrationSuite) setupRootfs() {
	var err error

	s.rootfs, err = os.MkdirTemp("", "containerd-integration")
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

func (s *IntegrationSuite) AfterTest(suiteName, testName string) {
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

// TestContainerCreateRunStopedDestroy validates that we're able to go through the
// whole lifecycle of:
//
// 1. creating the container
// 2. running a process in it
// 3. stopping the process
// 4. deleting the container
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
func (s *IntegrationSuite) TestContainerNetworkEgress() {
	handle := uuid()

	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		NetOut:     []garden.NetOutRule{{Log: true}},
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

	s.Equal(0, exitCode)
	s.Equal("200 OK\n", buf.String())
}

// TestHermeticContainerNetworkEgress aims at verifying that a process that we run in a
// container that we create through our gardenBackend is not able to make requests to
// external services when hermitc (that NetOut in container spec is empty) is true.
func (s *IntegrationSuite) TestHermeticContainerNetworkEgress() {
	handle := uuid()

	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		NetOut:     []garden.NetOutRule{},
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

	s.Equal(exitCode, 1, "Process in container should not be able to connect to external network")
	s.Contains(buf.String(), "failed performing http getGet \"http://example.com\": context deadline exceeded")
}

// TestContainerNetworkEgressWithRestrictedNetworks verifies that a process that we run in a
// container that we create through our gardenBackend is not able to reach an address that
// we have blocked access to.
func (s *IntegrationSuite) TestContainerNetworkEgressWithRestrictedNetworks() {
	// Using custom backend, clean up BeforeTest() stuff
	s.gardenBackend.Stop()
	s.cleanupIptables()

	namespace := "test-restricted-networks"
	requestTimeout := 3 * time.Second

	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithRestrictedNetworks([]string{"1.1.1.1"}),
	)

	s.NoError(err)

	customBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(requestTimeout),
	)
	s.NoError(err)

	s.NoError(customBackend.Start())

	handle := uuid()

	container, err := customBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		NetOut:     []garden.NetOutRule{{Log: true}},
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

// TestContainerBlocksHostAccess verifies that a process that we run in a
// container is not able to reach the host but is able to reach the internet.
func (s *IntegrationSuite) TestContainerBlocksHostAccess() {
	handle := uuid()
	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		NetOut:     []garden.NetOutRule{{Log: true}},
	})
	s.NoError(err)

	defer func() {
		s.NoError(s.gardenBackend.Destroy(handle))
		s.gardenBackend.Stop()
	}()

	hostIp, err := gateway.DiscoverInterface()
	s.NoError(err)

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	l, err := net.Listen("tcp", hostIp.String()+":0")
	ts.Listener = l
	ts.Start()
	defer ts.Close()

	buf := new(buffer)
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{
				"-http-get=" + ts.URL,
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
	s.Equal(exitCode, 1, "Process in container should not be able to connect to host network")

	proc, err = container.Run(
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

	exitCode, err = proc.Wait()
	s.NoError(err)
	s.Equal(0, exitCode, "Process in container should also be able to reach the internet")
}

func (s *IntegrationSuite) TestContainerAllowsHostAccess() {
	// Using custom backend, clean up BeforeTest() stuff
	s.gardenBackend.Stop()
	s.cleanupIptables()

	namespace := "test-allow-host-access"
	requestTimeout := 3 * time.Second

	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithAllowHostAccess(),
	)

	s.NoError(err)

	customBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(requestTimeout),
	)
	s.NoError(err)

	s.NoError(customBackend.Start())

	handle := uuid()

	container, err := customBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		NetOut:     []garden.NetOutRule{{Log: true}},
	})
	s.NoError(err)

	defer func() {
		s.NoError(customBackend.Destroy(handle))
		customBackend.Stop()
	}()

	hostIp, err := gateway.DiscoverInterface()
	s.NoError(err)

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	l, err := net.Listen("tcp", hostIp.String()+":0")
	ts.Listener = l
	ts.Start()
	defer ts.Close()

	buf := new(buffer)
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{
				"-http-get=" + ts.URL,
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
	s.Equal(0, exitCode, "Process in container should be able to reach the host network")
}

func (s *IntegrationSuite) TestContainerNetworkHosts() {
	s.NoError(s.gardenBackend.Start())

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
				"-cat=/etc/hosts",
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
	s.Contains(buf.String(), handle)
}

// TestRunPrivileged tests whether we're able to run a process in a privileged
// container.
func (s *IntegrationSuite) TestRunPrivileged() {
	s.runToCompletion(true)
}

// TestRunUnprivileged tests whether we're able to run a process in an
// unprivileged container.
//
// Differently from the privileged counterpart, we first need to change the
// ownership of the rootfs so the uid 0 inside the container has the permissions
// to execute the executable in there.
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

// TestRunWithoutTerminalStdinReturnsEOF validates that when running a process
// with TTY disabled, the stdin stream eventually sends an EOF (so stdin can be
// read to completion)
func (s *IntegrationSuite) TestRunWithoutTerminalStdinReturnsEOF() {
	handle := uuid()
	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	stdin := strings.NewReader("hello world")
	buf := new(buffer)

	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{
				"-cat=/dev/stdin",
			},
			// Only applies when TTY is nil
			TTY: nil,
		},
		garden.ProcessIO{
			Stdin:  stdin,
			Stdout: buf,
			Stderr: buf,
		},
	)
	s.NoError(err)

	exitCode, err := proc.Wait()
	s.NoError(err)

	s.Equal(exitCode, 0)
	s.Contains(buf.String(), "hello world")

	err = s.gardenBackend.Destroy(container.Handle())
	s.NoError(err)
}

// TestRunWithTerminalStdinClosed validates that when running a process with
// TTY enabled, if the stdin of of that process is closed (i.e. network flake),
// the process does not exit.
//
// Note that only (Concourse) tasks has TTY enabled
func (s *IntegrationSuite) TestRunWithTerminalStdinClosed() {
	handle := uuid()
	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	stdin := iotest.ErrReader(fmt.Errorf("Connection closed"))
	buf := new(buffer)

	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "/executable",
			Args: []string{
				"-sleep=5s",
			},
			TTY: &garden.TTYSpec{
				WindowSize: &garden.WindowSize{Columns: 500, Rows: 500},
			},
		},
		garden.ProcessIO{
			Stdin:  stdin,
			Stdout: buf,
			Stderr: buf,
		},
	)
	s.NoError(err)

	exitCode, err := proc.Wait()
	s.NoError(err)

	s.Equal(exitCode, 0)
	s.Contains(buf.String(), "slept for 5s")

	err = s.gardenBackend.Destroy(container.Handle())
	s.NoError(err)
}

// TestAttachToUnknownProc verifies that trying to attach to a process that does
// not exist lead to an error.
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
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	s.Error(err)
}

// TestAttach tries to validate that we're able to start a process in a
// container, get rid of the original client that originated the process, and
// then attach back to that process from a new client.
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
func (s *IntegrationSuite) TestCustomDNS() {
	// Using custom backend, clean up BeforeTest() stuff
	s.gardenBackend.Stop()
	s.cleanupIptables()

	namespace := "test-custom-dns"
	requestTimeout := 3 * time.Second

	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithNameServers([]string{
			"1.1.1.1", "1.2.3.4",
		}),
	)
	s.NoError(err)

	customBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(requestTimeout),
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
func (s *IntegrationSuite) TestUngracefulStop() {
	var ungraceful = true
	s.testStop(ungraceful)
}

// TestGracefulStop aims at validating that we're giving the process enough
// opportunity to finish, but that at the same time, we don't wait forever.
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

// TestMaxContainers aims at making sure that when the max container count is
// reached, any additional Create calls will fail
func (s *IntegrationSuite) TestMaxContainers() {
	// Using custom backend, clean up BeforeTest() stuff
	s.gardenBackend.Stop()
	s.cleanupIptables()

	namespace := "test-max-containers"
	requestTimeout := 3 * time.Second

	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
	)
	s.NoError(err)

	customBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		runtime.WithMaxContainers(1),
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(requestTimeout),
	)
	s.NoError(err)

	s.NoError(customBackend.Start())

	handle1 := uuid()
	handle2 := uuid()

	_, err = customBackend.Create(garden.ContainerSpec{
		Handle:     handle1,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	defer func() {
		s.NoError(customBackend.Destroy(handle1))
		customBackend.Stop()
	}()

	// not destroying handle2 as it is never successfully created
	_, err = customBackend.Create(garden.ContainerSpec{
		Handle:     handle2,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.Error(err)
	s.Contains(err.Error(), "max containers reached")
}

func (s *IntegrationSuite) TestRequestTimeoutZero() {
	// Using custom backend, clean up BeforeTest() stuff
	s.gardenBackend.Stop()
	s.cleanupIptables()

	namespace := "test-requesTimeout-zero"
	requestTimeout := time.Duration(0)

	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
	)
	s.NoError(err)

	customBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(requestTimeout),
	)
	s.NoError(err)

	s.NoError(customBackend.Start())

	_, err = customBackend.Containers(garden.Properties{})
	s.NoError(err)

	defer func() {
		customBackend.Stop()
	}()
}

// TestPropertiesGetChunked tests that we are able to store arbitrarily long
// properties, getting around containerd's label length restriction.
func (s *IntegrationSuite) TestPropertiesGetChunked() {
	handle := uuid()

	longString := ""
	for i := 0; i < 10000; i++ {
		longString += strconv.Itoa(i)
	}

	properties := garden.Properties{
		"long1": longString,
		// Concourse may try to set an empty value property on a container.
		// This just gets ignored (i.e. subsequent calls to
		// container.Properties() won't include it)
		"empty": "",
	}

	container, err := s.gardenBackend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
		Properties: properties,
	})
	s.NoError(err)

	containers, err := s.gardenBackend.Containers(garden.Properties{
		"long1": longString,
	})
	s.NoError(err)

	s.Len(containers, 1)

	err = container.SetProperty("long2", longString)
	s.NoError(err)

	containers, err = s.gardenBackend.Containers(garden.Properties{
		"long1": longString,
		"long2": longString,
	})
	s.NoError(err)
	s.Len(containers, 1)

	err = container.SetProperty(longString, "foo")
	s.Error(err)
	s.Regexp("property.*too long", err.Error())

	properties, err = container.Properties()
	s.NoError(err)

	s.Equal(garden.Properties{
		"long1": longString,
		"long2": longString,
	}, properties)
}

func (s *IntegrationSuite) TestNetworkMountsAreRemoved() {
	// Using custom backend, clean up BeforeTest() stuff
	s.gardenBackend.Stop()
	s.cleanupIptables()

	namespace := "test-network-mounts-are-removed"
	requestTimeout := 3 * time.Second

	networkMountsDir := s.T().TempDir()
	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNIFileStore(runtime.FileStoreWithWorkDir(networkMountsDir)),
	)
	s.NoError(err)

	customBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(
			s.containerdSocket(),
			namespace,
			requestTimeout,
		),
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(requestTimeout),
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

	networkFiles, err := os.ReadDir(filepath.Join(networkMountsDir, "networkmounts", handle))
	s.NoError(err)
	s.Len(networkFiles, 3)

	s.NoError(customBackend.Destroy(handle))

	networkFiles, err = os.ReadDir(filepath.Join(networkMountsDir, "networkmounts"))
	s.NoError(err)
	s.Len(networkFiles, 0)
}
