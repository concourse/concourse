package backend_test

import (
	"testing"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd/errdefs"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ContainerSuite struct {
	suite.Suite
	*require.Assertions

	backend backend.Backend
	fakeContainer *libcontainerdfakes.FakeContainer

	backendContainer garden.Container
}

const (
	namespaceForContainerUnitTest = "container-test"
	defaultTimeoutDurationForContainerUnitTest = 10 * time.Second
)

func (s *ContainerSuite) SetupTest() {
	s.fakeContainer = &libcontainerdfakes.FakeContainer{}
	s.backendContainer = backend.NewContainer(
		backend.ContainerdContext{
			Namespace: namespaceForContainerUnitTest,
			TimeoutDuration: defaultTimeoutDurationForContainerUnitTest,
		},
		s.fakeContainer)
}

func(s *ContainerSuite) TestStopUsesSIGTERM() {
	fakeTask := &libcontainerdfakes.FakeTask{}
	s.fakeContainer.TaskReturns(fakeTask, nil)
	err := s.backendContainer.Stop(false)
	s.NoError(err)
}

func(s *ContainerSuite) TestStopUsesSIGKILL() {
	fakeTask := &libcontainerdfakes.FakeTask{}
	s.fakeContainer.TaskReturns(fakeTask, nil)
	err := s.backendContainer.Stop(true)
	s.NoError(err)
}

func (s *ContainerSuite) TestStopWithoutTaskExisted() {
	s.fakeContainer.TaskReturns(nil, errdefs.ErrNotFound)
	err := s.backendContainer.Stop(true)
	s.NoError(err)
}

func TestSuite(t *testing.T) {
	suite.Run(t, &ContainerSuite{
		Assertions: require.New(t),
	})
}
