package backend_test

import (
	"context"
	"errors"
	"testing"

	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/backendfakes"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/containerd/typeurl"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KillerSuite struct {
	suite.Suite
	*require.Assertions

	task          *libcontainerdfakes.FakeTask
	processKiller *backendfakes.FakeProcessKiller
	killer        backend.Killer
}

func (s *KillerSuite) SetupTest() {
	s.task = new(libcontainerdfakes.FakeTask)
	s.processKiller = new(backendfakes.FakeProcessKiller)
	s.killer = backend.NewKiller(
		backend.WithProcessKiller(s.processKiller),
	)
}

func (s *KillerSuite) TestKillTaskWithNoProcs() {
	s.T().Run("graceful", func(_ *testing.T) {
		err := s.killer.Kill(context.Background(), s.task, false)
		s.NoError(err)

	})

	s.T().Run("ungraceful", func(_ *testing.T) {
		err := s.killer.Kill(context.Background(), s.task, true)
		s.NoError(err)
	})

	s.Equal(2, s.task.PidsCallCount())
	s.Equal(0, s.task.LoadProcessCallCount())
}

func (s *KillerSuite) TestKillTaskPidsErr() {
	expectedErr := errors.New("pids-err")
	s.task.PidsReturns(nil, expectedErr)

	s.T().Run("graceful", func(_ *testing.T) {
		err := s.killer.Kill(context.Background(), s.task, false)
		s.True(errors.Is(err, expectedErr))
	})

	s.T().Run("ungraceful", func(_ *testing.T) {
		err := s.killer.Kill(context.Background(), s.task, true)
		s.True(errors.Is(err, expectedErr))
	})
}

func (s *KillerSuite) TestKillTaskWithOnlyInitProc() {
	s.task.PidsReturns([]containerd.ProcessInfo{
		{Pid: 1234, Info: nil}, // the `init` proc returns `info: nil`
	}, nil)

	s.T().Run("graceful", func(_ *testing.T) {
		err := s.killer.Kill(context.Background(), s.task, true)
		s.NoError(err)
	})

	s.T().Run("ungraceful", func(_ *testing.T) {
		err := s.killer.Kill(context.Background(), s.task, true)
		s.NoError(err)
	})

	s.Equal(2, s.task.PidsCallCount())
	s.Equal(0, s.task.LoadProcessCallCount())
	s.Equal(0, s.processKiller.KillCallCount())
}

func (s *KillerSuite) TestKillTaskLoadProcessError() {
	procInfo, err := typeurl.MarshalAny(&options.ProcessDetails{
		ExecID: "execution-1",
	})
	s.NoError(err)

	s.task.PidsReturns([]containerd.ProcessInfo{
		{Pid: 123, Info: procInfo},
	}, nil)

	expectedErr := errors.New("load-proc-err")
	s.task.LoadProcessReturns(nil, expectedErr)

	s.T().Run("graceful", func(_ *testing.T) {
		err = s.killer.Kill(context.Background(), s.task, true)
		s.True(errors.Is(err, expectedErr))
	})

	s.T().Run("ungraceful", func(_ *testing.T) {
		err = s.killer.Kill(context.Background(), s.task, true)
		s.True(errors.Is(err, expectedErr))
	})
}

func (s *KillerSuite) TestUngracefulKillTaskProcKillError() {
	procInfo, err := typeurl.MarshalAny(&options.ProcessDetails{
		ExecID: "execution-1",
	})
	s.NoError(err)

	s.task.PidsReturns([]containerd.ProcessInfo{
		{Pid: 123, Info: procInfo},
	}, nil)

	expectedErr := errors.New("load-proc-err")
	s.processKiller.KillReturns(expectedErr)

	err = s.killer.Kill(context.Background(), s.task, true)
	s.True(errors.Is(err, expectedErr))
}

func (s *KillerSuite) TestGracefulKillTaskProcKillGracePeriodTimeoutError() {
	procInfo, err := typeurl.MarshalAny(&options.ProcessDetails{
		ExecID: "execution-1",
	})
	s.NoError(err)

	s.task.PidsReturns([]containerd.ProcessInfo{
		{Pid: 123, Info: procInfo},
	}, nil)

	expectedErr := backend.ErrGracePeriodTimeout
	s.processKiller.KillReturnsOnCall(0, expectedErr)

	err = s.killer.Kill(context.Background(), s.task, false)
	s.NoError(err)

	s.Equal(2, s.processKiller.KillCallCount())
}

func (s *KillerSuite) TestGracefulKillTaskProcKillUncaughtError() {
	procInfo, err := typeurl.MarshalAny(&options.ProcessDetails{
		ExecID: "execution-1",
	})
	s.NoError(err)

	s.task.PidsReturns([]containerd.ProcessInfo{
		{Pid: 123, Info: procInfo},
	}, nil)

	expectedErr := errors.New("kill-err")
	s.processKiller.KillReturnsOnCall(0, expectedErr)

	err = s.killer.Kill(context.Background(), s.task, false)
	s.True(errors.Is(err, expectedErr))

	s.Equal(1, s.processKiller.KillCallCount())
}

func (s *KillerSuite) TestGracefulKillTaskProcKillErrorOnUngracefulTry() {
	procInfo, err := typeurl.MarshalAny(&options.ProcessDetails{
		ExecID: "execution-1",
	})
	s.NoError(err)

	s.task.PidsReturns([]containerd.ProcessInfo{
		{Pid: 123, Info: procInfo},
	}, nil)

	s.processKiller.KillReturnsOnCall(0, backend.ErrGracePeriodTimeout)
	expectedErr := errors.New("ungraceful-kill-err")
	s.processKiller.KillReturnsOnCall(1, expectedErr)

	err = s.killer.Kill(context.Background(), s.task, false)
	s.True(errors.Is(err, expectedErr))

	s.Equal(2, s.processKiller.KillCallCount())
}
