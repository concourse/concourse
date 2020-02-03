package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd"
	"github.com/concourse/concourse/worker/workercmd"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackendSuite struct {
	suite.Suite
	*require.Assertions

	backend backend.Backend
	client  *libcontainerd.Client
}

func containerdRunner() ifrit.Runner {
	var (
		sock = filepath.Join(cmd.WorkDir.Path(), "containerd.sock")
		root = filepath.Join(cmd.WorkDir.Path(), "containerd")
		bin  = "containerd"
	)

	args := []string{
		"--address=" + sock,
		"--root=" + root,
	}

	if cmd.Garden.Config.Path() != "" {
		args = append(args, "--config", cmd.Garden.Config.Path())
	}

	if cmd.Garden.Bin != "" {
		bin = cmd.Garden.Bin
	}

	command := exec.Command(bin, args...)

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	return workercmd.CmdRunner{command}
}

func (s *BackendSuite) SetupSuite() {
	fmt.Println("sleeping")
	time.Sleep(1 * time.Second)
	fmt.Println("testing")
}

func (s *BackendSuite) SetupTest() {
	namespace := "test" + strconv.FormatInt(time.Now().UnixNano(), 10)

	s.backend = backend.New(
		libcontainerd.New("/run/containerd/containerd.sock"),
		namespace,
	)

	s.NoError(s.backend.Start())
}

func (s *BackendSuite) TearDownTest() {
	s.backend.Stop()
}

func (s *BackendSuite) TestPing() {
	s.NoError(s.backend.Ping())
}

func (s *BackendSuite) TestContainerCreateAndDestroy() {
	handle := mustCreateHandle()
	rootfs, err := filepath.Abs("testdata/rootfs")
	s.NoError(err)

	_, err = s.backend.Create(garden.ContainerSpec{
		Handle:     handle,
		RootFSPath: "raw://" + rootfs,
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

func TestSuite(t *testing.T) {
	suite.Run(t, &BackendSuite{
		Assertions: require.New(t),
	})
}

func (s *BackendSuite) TearDownSuite() {
	fmt.Println("tearing down")
}

func mustCreateHandle() string {
	u4, err := uuid.NewV4()
	if err != nil {
		panic("couldn't create new uuid")
	}

	return u4.String()
}
