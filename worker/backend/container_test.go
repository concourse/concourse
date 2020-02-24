package backend_test

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/backendfakes"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/typeurl"
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

// func (s *ContainerSuite) TestStopWithKillUngracefullyStops() {
// 	err := s.container.Stop(true)
// 	s.NoError(err)
// 	s.Equal(1, s.ungracefulKiller.KillCallCount())
// }

// func (s *ContainerSuite) TestStopWithKillFailing() {
// 	s.ungracefulKiller.UngracefullyStopReturns(errors.New("ungraceful-stop-err"))

// 	err := s.container.Stop(true)
// 	s.Equal(1, s.ungracefulKiller.UngracefullyStopCallCount())
// 	s.EqualError(errors.Unwrap(err), "ungraceful-stop-err")
// }

// func (s *ContainerSuite) TestStopWithoutKillGracefullyStops() {
// 	err := s.container.Stop(false)
// 	s.NoError(err)
// 	s.Equal(1, s.ungracefulKiller.GracefullyStopCallCount())
// }

// func (s *ContainerSuite) TestStopWithoutKillFailing() {
// 	s.ungracefulKiller.GracefullyStopReturns(errors.New("graceful-stop-err"))

// 	err := s.container.Stop(false)
// 	s.EqualError(errors.Unwrap(err), "graceful-stop-err")
// 	s.Equal(1, s.ungracefulKiller.GracefullyStopCallCount())
// }

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

func (s *ContainerSuite) TestSetGraceTimeSetLabelsFails() {
	expectedErr := errors.New("set-label-error")
	s.containerdContainer.SetLabelsReturns(nil, expectedErr)

	err := s.container.SetGraceTime(1234)
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestSetGraceTimeSetLabelsSucceeds() {
	err := s.container.SetGraceTime(1234)
	s.NoError(err)

	expectedLabelSet := map[string]string{
		"garden.grace-time": "1234",
	}
	_, labelSet := s.containerdContainer.SetLabelsArgsForCall(0)
	s.Equal(expectedLabelSet, labelSet)
}

func (s *ContainerSuite) TestPropertyGetLabelsFails() {
	expectedErr := errors.New("get-labels-error")
	s.containerdContainer.LabelsReturns(nil, expectedErr)
	_, err := s.container.Property("any")
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestPropertyNotFound() {
	s.containerdContainer.LabelsReturns(garden.Properties{}, nil)
	_, err := s.container.Property("any")
	s.Equal(backend.ErrNotFound("any"), err)
}

func (s *ContainerSuite) TestPropertyReturnsValue() {
	properties := garden.Properties{
		"any": "some-value",
	}
	s.containerdContainer.LabelsReturns(properties, nil)
	result, err := s.container.Property("any")
	s.NoError(err)
	s.Equal("some-value", result)
}

func (s *ContainerSuite) TestCurrentCPULimitsGetInfoFails() {
	expectedErr := errors.New("get-info-error")
	s.containerdContainer.InfoReturns(containers.Container{}, expectedErr)
	_, err := s.container.CurrentCPULimits()
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestCurrentCPULimitsNoSpec() {
	s.containerdContainer.InfoReturns(containers.Container{}, nil)
	limits, err := s.container.CurrentCPULimits()
	s.NoError(err)
	s.Equal(garden.CPULimits{}, limits)
}

func (s *ContainerSuite) TestCurrentCPULimitsNoCPUShares() {
	spec, err := typeurl.MarshalAny(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{},
			},
		},
	})
	s.NoError(err)

	s.containerdContainer.InfoReturns(containers.Container{
		Spec: spec,
	}, nil)
	limits, err := s.container.CurrentCPULimits()
	s.NoError(err)
	s.Equal(garden.CPULimits{}, limits)
}

func (s *ContainerSuite) TestCurrentCPULimitsReturnsCPUShares() {
	cpuShares := uint64(512)
	spec, err := typeurl.MarshalAny(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Shares: &cpuShares,
				},
			},
		},
	})
	s.NoError(err)

	s.containerdContainer.InfoReturns(containers.Container{
		Spec: spec,
	}, nil)
	limits, err := s.container.CurrentCPULimits()
	s.NoError(err)
	s.Equal(garden.CPULimits{Weight: cpuShares}, limits)
}

func (s *ContainerSuite) TestCurrentMemoryLimitsGetInfoFails() {
	expectedErr := errors.New("get-info-error")
	s.containerdContainer.InfoReturns(containers.Container{}, expectedErr)
	_, err := s.container.CurrentMemoryLimits()
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestCurrentMemoryLimitsNoSpec() {
	s.containerdContainer.InfoReturns(containers.Container{}, nil)
	limits, err := s.container.CurrentMemoryLimits()
	s.NoError(err)
	s.Equal(garden.MemoryLimits{}, limits)
}

func (s *ContainerSuite) TestCurrentMemoryLimitsNoCPUShares() {
	spec, err := typeurl.MarshalAny(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{},
			},
		},
	})
	s.NoError(err)

	s.containerdContainer.InfoReturns(containers.Container{
		Spec: spec,
	}, nil)
	limits, err := s.container.CurrentMemoryLimits()
	s.NoError(err)
	s.Equal(garden.MemoryLimits{}, limits)
}

func (s *ContainerSuite) TestCurrentMemoryLimitsReturnsLimit() {
	limitBytes := int64(512)
	spec, err := typeurl.MarshalAny(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: &limitBytes,
				},
			},
		},
	})
	s.NoError(err)

	s.containerdContainer.InfoReturns(containers.Container{
		Spec: spec,
	}, nil)
	limits, err := s.container.CurrentMemoryLimits()
	s.NoError(err)
	s.Equal(garden.MemoryLimits{LimitInBytes: uint64(limitBytes)}, limits)
}
