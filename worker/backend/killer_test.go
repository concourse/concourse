package backend_test

import (
	"context"
	"errors"
	"time"

	"github.com/concourse/concourse/worker/backend"
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

	task             *libcontainerdfakes.FakeTask
	ungracefulKiller backend.Killer
	gracefulKiller   backend.Killer
}

func (s *KillerSuite) SetupTest() {
	s.task = new(libcontainerdfakes.FakeTask)
	s.ungracefulKiller = backend.NewUngracefulKiller()
	s.gracefulKiller = backend.NewGracefulKiller()
}

func (s *KillerSuite) TestUngracefulKillWaitErr() {
	s.task.WaitReturns(nil, errors.New("wait-err"))

	err := s.ungracefulKiller.Kill(context.Background(), s.task)
	s.EqualError(errors.Unwrap(errors.Unwrap(err)), "wait-err")
}

func (s *KillerSuite) TestUngracefulKillKillErr() {
	s.task.KillReturns(errors.New("kill-err"))

	err := s.ungracefulKiller.Kill(context.Background(), s.task)
	s.EqualError(errors.Unwrap(errors.Unwrap(err)), "kill-err")
}

func (s *KillerSuite) TestUngracefulKillContextErrWhileWaiting() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.ungracefulKiller.Kill(ctx, s.task)
	s.EqualError(errors.Unwrap(err), "ctx done: context canceled")
}

func (s *KillerSuite) TestUngracefulKillWaitsWithContextHavingDeadlineSet() {
	ch := make(chan containerd.ExitStatus, 1)
	ch <- *containerd.NewExitStatus(0, time.Now(), nil)
	s.task.WaitReturns(ch, nil)

	err := s.ungracefulKiller.Kill(context.Background(), s.task)
	s.NoError(err)

	s.Equal(1, s.task.WaitCallCount())
	ctx := s.task.WaitArgsForCall(0)

	_, deadlineIsSet := ctx.Deadline()
	s.True(deadlineIsSet)
}

func (s *KillerSuite) TestGracefulKillErrorListingExecedPids() {
	expectedErr := errors.New("pids-err")
	s.task.PidsReturns(nil, expectedErr)

	err := s.gracefulKiller.Kill(context.Background(), s.task)
	s.True(errors.Is(err, expectedErr))
}

func (s *KillerSuite) TestGracefulKillErrorLoadingExecedProc() {
	procInfo, err := typeurl.MarshalAny(&options.ProcessDetails{
		ExecID: "execution-1",
	})
	s.NoError(err)

	s.task.PidsReturns([]containerd.ProcessInfo{
		{
			Pid:  123,
			Info: procInfo,
		},
	}, nil)

	expectedErr := errors.New("load-proc-err")
	s.task.LoadProcessReturns(nil, expectedErr)

	err = s.gracefulKiller.Kill(context.Background(), s.task)
	s.True(errors.Is(err, expectedErr))
}

// TODO verify that we actually try to kill
// - we could have a "ProcessKiller` iface ... or something ... (so that we
//   don't duplicate our testing here) - in the end, both killers end up using
//   the same idea (they just target different processes)
//
