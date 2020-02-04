package integration_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/containerd/containerd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tedsuo/ifrit"
)

type IntegrationSuite struct {
	suite.Suite
	*require.Assertions

	backend           backend.Backend
	rootfs            string
	client            *libcontainerd.Client
	containerdProcess ifrit.Process
	tmpDir            string
	stdout            bytes.Buffer
	stderr            bytes.Buffer
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

func (s *IntegrationSuite) SetupTest() {
	var (
		err            error
		namespace      = "test" + strconv.FormatInt(time.Now().UnixNano(), 10)
		requestTimeout = 3 * time.Second
	)

	// TODO - have `init` being compiled from here

	s.backend, err = backend.New(
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

func (s *IntegrationSuite) setupRootfs() (err error) {
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

func (s *IntegrationSuite) TestContainerCreateAndDestroy() {
	handle := mustCreateHandle()

	_, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	containers, err := s.backend.Containers(nil)
	s.NoError(err)

	s.Len(containers, 1)

	err = s.backend.Destroy(handle)
	s.NoError(err)

	containers, err = s.backend.Containers(nil)
	s.NoError(err)
	s.Len(containers, 0)
}

func (s *IntegrationSuite) TestContainerNetworkEgress() {
	handle := mustCreateHandle()

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

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

	err = s.backend.Destroy(container.Handle())
	s.NoError(err)

}

func (s *IntegrationSuite) TestContainerRunPrivilegedToCompletion() {
	s.runToCompletion(true)
}

// go test -run TestSuite/TestContainerRunUnprivilegedToCompletion
func (s *IntegrationSuite) TestContainerRunUnprivilegedToCompletion() {
	maxUid, maxGid, err := backend.NewUserNamespace().MaxValidIds()
	s.NoError(err)

	filepath.Walk(s.rootfs, func(path string, _ os.FileInfo, _ error) error {
		return os.Lchown(path, int(maxUid), int(maxGid))
	})

	s.runToCompletion(false)
}

func (s *IntegrationSuite) TearDownSuite() {
	s.containerdProcess.Signal(syscall.SIGTERM)
	<-s.containerdProcess.Wait()
}

func (s *IntegrationSuite) runToCompletion(privileged bool) {
	handle := mustCreateHandle()

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: privileged,
	})
	s.NoError(err)

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

	err = s.backend.Destroy(container.Handle())
	s.NoError(err)
}

func (s *IntegrationSuite) TestAttachToUnknownProc() {
	handle := mustCreateHandle()

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

	_, err = container.Attach("inexistent", garden.ProcessIO{
		Stdout: ioutil.Discard,
		Stderr: ioutil.Discard,
	})
	s.Error(err)
}

func (s *IntegrationSuite) TestAttach() {
	s.T().Skip()
	handle := mustCreateHandle()

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

// TestContainerWaitGracefulStop aims at validating that we're giving the
// process enough opportunity to finish, but that we don't wait forever.
//
func (s *IntegrationSuite) TestContainerWaitGracefulStop() {
	handle := mustCreateHandle()
	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + s.rootfs,
		Privileged: true,
	})
	s.NoError(err)

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

	err = s.backend.Destroy(container.Handle())
	s.NoError(err)
}
