package integration_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
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
	"github.com/containerd/containerd"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tedsuo/ifrit"
)

type BackendSuite struct {
	suite.Suite
	*require.Assertions

	backend           backend.Backend
	client            *libcontainerd.Client
	containerdProcess ifrit.Process
	tmpDir            string
	stdout            bytes.Buffer
	stderr            bytes.Buffer
}

func (s *BackendSuite) containerdSocket() string {
	return filepath.Join(s.tmpDir, "containerd.sock")
}

func (s *BackendSuite) startContainerd() {
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

func (s *BackendSuite) SetupSuite() {
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

func (s *BackendSuite) SetupTest() {
	namespace := "test" + strconv.FormatInt(time.Now().UnixNano(), 10)

	s.backend = backend.New(
		libcontainerd.New(s.containerdSocket()),
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
	tmpDir, err := ioutil.TempDir("", "containerd-test")
	if err != nil {
		panic(err)
	}
	suite.Run(t, &BackendSuite{
		Assertions: require.New(t),
		tmpDir:     tmpDir,
	})
}

func (s *BackendSuite) TearDownSuite() {
	s.containerdProcess.Signal(syscall.SIGTERM)
	<-s.containerdProcess.Wait()
}

func mustCreateHandle() string {
	u4, err := uuid.NewV4()
	if err != nil {
		panic("couldn't create new uuid")
	}

	return u4.String()
}
