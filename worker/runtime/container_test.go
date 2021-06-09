package runtime_test

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd/libcontainerdfakes"
	"github.com/concourse/concourse/worker/runtime/runtimefakes"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ContainerSuite struct {
	suite.Suite
	*require.Assertions

	container           *runtime.Container
	containerdContainer *libcontainerdfakes.FakeContainer
	containerdProcess   *libcontainerdfakes.FakeProcess
	containerdTask      *libcontainerdfakes.FakeTask
	rootfsManager       *runtimefakes.FakeRootfsManager
	killer              *runtimefakes.FakeKiller
}

func (s *ContainerSuite) SetupTest() {
	s.containerdContainer = new(libcontainerdfakes.FakeContainer)
	s.containerdProcess = new(libcontainerdfakes.FakeProcess)
	s.containerdTask = new(libcontainerdfakes.FakeTask)
	s.rootfsManager = new(runtimefakes.FakeRootfsManager)
	s.killer = new(runtimefakes.FakeKiller)

	s.container = runtime.NewContainer(
		s.containerdContainer,
		s.killer,
		s.rootfsManager,
	)
}

func (s *ContainerSuite) TestStopWithKillUngracefullyStops() {
	err := s.container.Stop(true)
	s.NoError(err)
	s.Equal(1, s.killer.KillCallCount())
	_, _, behaviour := s.killer.KillArgsForCall(0)
	s.Equal(runtime.KillUngracefully, behaviour)
}

func (s *ContainerSuite) TestStopWithKillGracefullyStops() {
	err := s.container.Stop(false)
	s.NoError(err)
	s.Equal(1, s.killer.KillCallCount())
	_, _, behaviour := s.killer.KillArgsForCall(0)
	s.Equal(runtime.KillGracefully, behaviour)
}

func (s *ContainerSuite) TestStopErrorsTaskLookup() {
	expectedErr := errors.New("task-lookup-err")
	s.containerdContainer.TaskReturns(nil, expectedErr)

	err := s.container.Stop(false)
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestStopErrorsKill() {
	expectedErr := errors.New("kill-err")
	s.killer.KillReturns(expectedErr)

	err := s.container.Stop(false)
	s.True(errors.Is(err, expectedErr))
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
		Root:    &specs.Root{},
	}, nil)

	expectedErr := errors.New("setup-cwd-err")
	s.rootfsManager.SetupCwdReturns(expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{Dir: "/somewhere"}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunTaskError() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	expectedErr := errors.New("task-err")
	s.containerdContainer.TaskReturns(nil, expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunTaskExecError() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
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
		Root:    &specs.Root{},
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
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedErr := errors.New("proc-start-err")
	s.containerdProcess.StartReturns(expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunProcStartErrorExecutableNotFound() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	exeNotFoundErr := errors.New("OCI runtime exec failed: exec failed: container_linux.go:345: starting container process caused: exec: potato: executable file not found in $PATH")
	s.containerdProcess.StartReturns(exeNotFoundErr)

	_, err := s.container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	s.True(errors.Is(err, garden.ExecutableNotFoundError{Message: exeNotFoundErr.Error()}))
}

func (s *ContainerSuite) TestRunWithUserLookupSucceeds() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedUser := specs.User{UID: 1, GID: 2, Username: "some_user"}
	s.rootfsManager.LookupUserReturns(expectedUser, true, nil)

	_, err := s.container.Run(garden.ProcessSpec{User: "some_user"}, garden.ProcessIO{})
	s.NoError(err)

	_, _, procSpec, _ := s.containerdTask.ExecArgsForCall(0)
	s.Equal(expectedUser, procSpec.User)

	userEnvVarSet := false
	expectedEnvVar := "USER=some_user"

	for _, envVar := range procSpec.Env {
		if envVar == expectedEnvVar {
			userEnvVarSet = true
			break
		}
	}
	s.True(userEnvVarSet)
}

func (s *ContainerSuite) TestDoesNotOverwriteExistingPathInImage() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedUser := specs.User{UID: 0, GID: 0, Username: "root"}
	s.rootfsManager.LookupUserReturns(expectedUser, true, nil)

	expectedImagePath := "PATH=/usr/local/image-path"
	_, err := s.container.Run(
		garden.ProcessSpec{
			User: "root",
			Env:  []string{expectedImagePath},
		},
		garden.ProcessIO{},
	)
	s.NoError(err)

	_, _, procSpec, _ := s.containerdTask.ExecArgsForCall(0)
	s.Equal(expectedUser, procSpec.User)

	userEnvVarSet := false

	for _, envVar := range procSpec.Env {
		if envVar == expectedImagePath {
			userEnvVarSet = true
			break
		}
	}
	s.True(userEnvVarSet)
}

func (s *ContainerSuite) TestRunWithRootUserHasSuperUserPath() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedUser := specs.User{UID: 0, GID: 0, Username: "root"}
	s.rootfsManager.LookupUserReturns(expectedUser, true, nil)

	_, err := s.container.Run(garden.ProcessSpec{User: "root"}, garden.ProcessIO{})
	s.NoError(err)

	_, _, procSpec, _ := s.containerdTask.ExecArgsForCall(0)
	s.Equal(expectedUser, procSpec.User)

	userEnvVarSet := false
	expectedEnvVar := runtime.SuperuserPath

	for _, envVar := range procSpec.Env {
		if envVar == expectedEnvVar {
			userEnvVarSet = true
			break
		}
	}
	s.True(userEnvVarSet)
}

func (s *ContainerSuite) TestRunWithNonRootUserHasUserPath() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedUser := specs.User{UID: 6, GID: 6, Username: "games"}
	s.rootfsManager.LookupUserReturns(expectedUser, true, nil)

	_, err := s.container.Run(garden.ProcessSpec{User: "games"}, garden.ProcessIO{})
	s.NoError(err)

	_, _, procSpec, _ := s.containerdTask.ExecArgsForCall(0)
	s.Equal(expectedUser, procSpec.User)

	userEnvVarSet := false
	expectedEnvVar := runtime.Path

	for _, envVar := range procSpec.Env {
		if envVar == expectedEnvVar {
			userEnvVarSet = true
			break
		}
	}
	s.True(userEnvVarSet)
}

func (s *ContainerSuite) TestRunWithUserLookupErrors() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	expectedErr := errors.New("lookup error")
	s.rootfsManager.LookupUserReturns(specs.User{}, false, expectedErr)

	_, err := s.container.Run(garden.ProcessSpec{User: "some_user"}, garden.ProcessIO{})
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestRunWithUserLookupNotFound() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}, nil)

	s.containerdContainer.TaskReturns(s.containerdTask, nil)
	s.containerdTask.ExecReturns(s.containerdProcess, nil)

	s.rootfsManager.LookupUserReturns(specs.User{}, false, nil)

	_, err := s.container.Run(garden.ProcessSpec{User: "some_invalid_user"}, garden.ProcessIO{})
	s.True(errors.Is(err, runtime.UserNotFoundError{User: "some_invalid_user"}))
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
	s.Equal(runtime.ErrNotFound("any"), err)
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
	expectedErr := errors.New("get-spec-error")
	s.containerdContainer.SpecReturns(nil, expectedErr)
	_, err := s.container.CurrentCPULimits()
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestCurrentCPULimitsNoLimitSet() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{},
			},
		},
	}, nil)
	limits, err := s.container.CurrentCPULimits()
	s.NoError(err)
	s.Equal(garden.CPULimits{}, limits)
}

func (s *ContainerSuite) TestCurrentCPULimitsReturnsCPUShares() {
	cpuShares := uint64(512)
	s.containerdContainer.SpecReturns(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Shares: &cpuShares,
				},
			},
		},
	}, nil)
	limits, err := s.container.CurrentCPULimits()
	s.NoError(err)
	s.Equal(garden.CPULimits{Weight: cpuShares}, limits)
}

func (s *ContainerSuite) TestCurrentMemoryLimitsGetSpecFails() {
	expectedErr := errors.New("get-spec-error")
	s.containerdContainer.SpecReturns(nil, expectedErr)
	_, err := s.container.CurrentMemoryLimits()
	s.True(errors.Is(err, expectedErr))
}

func (s *ContainerSuite) TestCurrentMemoryLimitsNoLimitSet() {
	s.containerdContainer.SpecReturns(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{},
			},
		},
	}, nil)
	limits, err := s.container.CurrentMemoryLimits()
	s.NoError(err)
	s.Equal(garden.MemoryLimits{}, limits)
}

func (s *ContainerSuite) TestCurrentMemoryLimitsReturnsLimit() {
	limitBytes := int64(512)
	s.containerdContainer.SpecReturns(&specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: &limitBytes,
				},
			},
		},
	}, nil)
	limits, err := s.container.CurrentMemoryLimits()
	s.NoError(err)
	s.Equal(garden.MemoryLimits{LimitInBytes: uint64(limitBytes)}, limits)
}
