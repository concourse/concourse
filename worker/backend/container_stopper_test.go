package backend_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/backendfakes"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd/errdefs"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ContainerStopperSuite struct {
	suite.Suite
	*require.Assertions

	fakeKiller    *backendfakes.FakeKiller
	fakeContainer *libcontainerdfakes.FakeContainer
	fakeTask      *libcontainerdfakes.FakeTask
}

func (s *ContainerStopperSuite) SetupTest() {
	s.fakeKiller = new(backendfakes.FakeKiller)
	s.fakeContainer = new(libcontainerdfakes.FakeContainer)
	s.fakeTask = new(libcontainerdfakes.FakeTask)
}

func (s *ContainerStopperSuite) TestStopGetTaskNotFoundErr() {
	s.fakeContainer.TaskReturns(nil, errdefs.ErrNotFound)

	err := backend.Stop(context.Background(), s.fakeContainer, s.fakeKiller)
	s.NoError(err)
}

func (s *ContainerStopperSuite) TestStopGetTaskError() {
	s.fakeContainer.TaskReturns(nil, errors.New("get-task-error"))

	err := backend.Stop(context.Background(), s.fakeContainer, s.fakeKiller)
	s.EqualError(errors.Unwrap(err), "get-task-error")
}

func (s *ContainerStopperSuite) TestStopKillerError() {
	s.fakeKiller.KillReturns(errors.New("killer-err"))

	err := backend.Stop(context.Background(), s.fakeContainer, s.fakeKiller)
	s.EqualError(errors.Unwrap(err), "killer-err")
}

func (s *ContainerStopperSuite) TestStopDeleteError() {
	s.fakeContainer.TaskReturns(s.fakeTask, nil)
	s.fakeTask.DeleteReturns(nil, errors.New("delete-err"))

	err := backend.Stop(context.Background(), s.fakeContainer, s.fakeKiller)
	s.EqualError(errors.Unwrap(err), "delete-err")
}
