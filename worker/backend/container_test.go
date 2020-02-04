package backend_test

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/backendfakes"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ContainerSuite struct {
	suite.Suite
	*require.Assertions

	containerdTask      *libcontainerdfakes.FakeTask
	containerdProcess   *libcontainerdfakes.FakeProcess
	containerdContainer *libcontainerdfakes.FakeContainer
	containerStopper    *backendfakes.FakeContainerStopper
	rootfsManager       *backendfakes.FakeRootfsManager
	container           *backend.Container
}

func (s *ContainerSuite) SetupTest() {
	s.containerStopper = new(backendfakes.FakeContainerStopper)
	s.rootfsManager = new(backendfakes.FakeRootfsManager)
	s.containerdContainer = new(libcontainerdfakes.FakeContainer)
	s.containerdTask = new(libcontainerdfakes.FakeTask)
	s.containerdProcess = new(libcontainerdfakes.FakeProcess)

	s.container = backend.NewContainer(
		s.containerdContainer,
		s.containerStopper,
		s.rootfsManager,
	)
}

func (s *ContainerSuite) TestDeleteWithKillUngracefullyStops() {
	err := s.container.Stop(true)
	s.NoError(err)
	s.Equal(1, s.containerStopper.UngracefullyStopCallCount())

}

func (s *ContainerSuite) TestDeleteWithKillFailing() {
	s.containerStopper.UngracefullyStopReturns(errors.New("ungraceful-stop-err"))

	err := s.container.Stop(true)
	s.Equal(1, s.containerStopper.UngracefullyStopCallCount())
	s.EqualError(errors.Unwrap(err), "ungraceful-stop-err")
}

func (s *ContainerSuite) TestDeleteWithoutKillGracefullyStops() {
	err := s.container.Stop(false)
	s.NoError(err)
	s.Equal(1, s.containerStopper.GracefullyStopCallCount())
}

func (s *ContainerSuite) TestDeleteWithoutKillFailing() {
	s.containerStopper.GracefullyStopReturns(errors.New("graceful-stop-err"))

	err := s.container.Stop(false)
	s.EqualError(errors.Unwrap(err), "graceful-stop-err")
	s.Equal(1, s.containerStopper.GracefullyStopCallCount())
}

func (s *ContainerSuite) TestRunContainerSpecErr() {
	expectedErr := errors.New("spec-err")
	s.containerdContainer.SpecReturns(nil, expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunWithNonRootCwdSetupCwdFails() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
	}, nil)

	expectedErr := errors.New("setup-cwd-err")
	s.rootfsManager.SetupCwdReturns(expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{Dir: "/somewhere"}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunTaskError() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
	}, nil)

	expectedErr := errors.New("task-err")
	s.containerdContainer.TaskReturns(nil, expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunTaskExecError() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)

	expectedErr := errors.New("exec-err")
	s.containerdTask.ExecReturns(nil, expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunProcWaitError() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedErr := errors.New("proc-wait-err")
	s.containerdProcess.WaitReturns(nil, expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunProcStartError() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedErr := errors.New("proc-start-err")
	s.containerdProcess.StartReturns(expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunProcCloseIOError() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedErr := errors.New("proc-closeio-err")
	s.containerdProcess.CloseIOReturns(expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunProcCloseIOWithStdin() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.NoError(err)

	s.Equal(1, s.containerdProcess.CloseIOCallCount())
	_, opts := s.containerdProcess.CloseIOArgsForCall(0)
	s.Len(opts, 1)

	// you can't compare two functions in Go, so, compare its effects (these
	// are functional opts).
	//
	obj := containerd.IOCloseInfo{}
	opts[0](&obj)

	// we want to make sure that we're passing an opt that sets `Stdin` to
	// true on an `IOCloseInfo`.
	s.True(obj.Stdin)
}
