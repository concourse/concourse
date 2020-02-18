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

	container           *backend.Container
	containerdContainer *libcontainerdfakes.FakeContainer
	containerdProcess   *libcontainerdfakes.FakeProcess
	containerdTask      *libcontainerdfakes.FakeTask
	rootfsManager       *backendfakes.FakeRootfsManager
	killer              *backendfakes.FakeKiller
}

func (s *ContainerSuite) SetupTest() {
	s.containerdContainer = new(libcontainerdfakes.FakeContainer)
	s.containerdProcess = new(libcontainerdfakes.FakeProcess)
	s.containerdTask = new(libcontainerdfakes.FakeTask)
	s.rootfsManager = new(backendfakes.FakeRootfsManager)
	s.killer = new(backendfakes.FakeKiller)

	s.container = backend.NewContainer(
		s.containerdContainer,
		s.killer,
		s.rootfsManager,
	)
}

func (s *ContainerSuite) TestStopLoadTaskFails() {
	expectedError := errors.New("load-task-fails")
	s.containerdContainer.TaskReturns(nil, expectedError)

	err := s.container.Stop(true)
	s.True(errors.Is(err, expectedError))
}

func (s *ContainerSuite) TestStopKillTaskFails() {
	expectedError := errors.New("kill-task-fails")
	s.killer.KillReturns(expectedError)

	err := s.container.Stop(true)
	s.True(errors.Is(err, expectedError))
}

func (s *ContainerSuite) TestStopKillGracefullySucceeds() {
	const kill = false
	err := s.container.Stop(kill)
	s.NoError(err)
	_, _, behaviour := s.killer.KillArgsForCall(0)
	s.Equal(backend.KillGracefully, behaviour)
}

func (s *ContainerSuite) TestStopKillUngracefullySucceeds() {
	const kill = true
	err := s.container.Stop(kill)
	s.NoError(err)
	_, _, behaviour := s.killer.KillArgsForCall(0)
	s.Equal(backend.KillUngracefully, behaviour)
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
