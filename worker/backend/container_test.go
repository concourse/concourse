package backend_test

import (
	"context"
	"errors"
	"syscall"
	"testing"

	"github.com/containerd/containerd/errdefs"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ContainerSuite struct {
	suite.Suite
	*require.Assertions

	backend       backend.Backend
	fakeContainer *libcontainerdfakes.FakeContainer

	backendContainer garden.Container
}

func (s *ContainerSuite) SetupTest() {
	s.fakeContainer = &libcontainerdfakes.FakeContainer{}
	s.backendContainer = backend.NewContainer(context.TODO(), s.fakeContainer)
}

func (s *ContainerSuite) TestStopNonexistentTask() {
	s.fakeContainer.TaskReturns(nil, errdefs.ErrNotFound)
	err := s.backendContainer.Stop(true)

	s.NoError(err)
}

func (s *ContainerSuite) TestStopUsesSIGTERM() {
	fakeTask := &libcontainerdfakes.FakeTask{}
	s.fakeContainer.TaskReturns(fakeTask, nil)
	err := s.backendContainer.Stop(false)
	_, signal, _ := fakeTask.KillArgsForCall(0)

	s.Equal(signal, syscall.SIGTERM)
	s.NoError(err)
}

func (s *ContainerSuite) TestStopUsesSIGKILL() {
	fakeTask := &libcontainerdfakes.FakeTask{}
	s.fakeContainer.TaskReturns(fakeTask, nil)
	err := s.backendContainer.Stop(true)
	_, signal, _ := fakeTask.KillArgsForCall(0)

	s.Equal(signal, syscall.SIGKILL)
	s.NoError(err)
}

func (s *ContainerSuite) TestStopKillTaskError() {
	fakeTask := &libcontainerdfakes.FakeTask{}
	s.fakeContainer.TaskReturns(fakeTask, nil)
	fakeTask.KillReturns(errors.New("task-kill-error"))

	err := s.backendContainer.Stop(false)
	s.EqualError(err, "task-kill-error")
}

func TestSuite(t *testing.T) {
	suite.Run(t, &ContainerSuite{
		Assertions: require.New(t),
	})
}
