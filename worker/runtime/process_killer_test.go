package runtime_test

import (
	"context"
	"errors"
	"math"
	"syscall"
	"time"

	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProcessKillerSuite struct {
	suite.Suite
	*require.Assertions

	signal syscall.Signal
	proc   *libcontainerdfakes.FakeProcess
	killer runtime.ProcessKiller

	goodEnoughTimeout time.Duration
	notEnoughTimeout  time.Duration
}

func (s *ProcessKillerSuite) SetupTest() {
	s.proc = new(libcontainerdfakes.FakeProcess)
	s.killer = runtime.NewProcessKiller()

	s.signal = 142
	s.goodEnoughTimeout = math.MaxInt64
	s.notEnoughTimeout = time.Nanosecond
}

func (s *ProcessKillerSuite) TestKillCallsWaitWithDeadline() {
	ch := make(chan containerd.ExitStatus, 1)
	ch <- *containerd.NewExitStatus(0, time.Now(), nil)
	s.proc.WaitReturns(ch, nil)

	err := s.killer.Kill(context.Background(), s.proc, s.signal, s.goodEnoughTimeout)
	s.NoError(err)

	s.Equal(1, s.proc.WaitCallCount())
	waitCtx := s.proc.WaitArgsForCall(0)
	_, deadlineSet := waitCtx.Deadline()
	s.True(deadlineSet)
}

func (s *ProcessKillerSuite) TestKillWaitError() {
	expectedErr := errors.New("wait-err")
	s.proc.WaitReturns(nil, expectedErr)

	err := s.killer.Kill(context.Background(), s.proc, s.signal, s.goodEnoughTimeout)
	s.True(errors.Is(err, expectedErr))
}

func (s *ProcessKillerSuite) TestKillKillError() {
	expectedErr := errors.New("kill-err")
	s.proc.KillReturns(expectedErr)

	err := s.killer.Kill(context.Background(), s.proc, s.signal, s.goodEnoughTimeout)
	s.True(errors.Is(err, expectedErr))
}

func (s *ProcessKillerSuite) TestKillWaitContextDeadlineReached() {
	err := s.killer.Kill(context.Background(), s.proc, s.signal, s.notEnoughTimeout)
	s.True(errors.Is(err, runtime.ErrGracePeriodTimeout))
}

func (s *ProcessKillerSuite) TestKillWaitContextCancelled() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.killer.Kill(ctx, s.proc, s.signal, s.goodEnoughTimeout)
	s.True(errors.Is(err, context.Canceled))
}

func (s *ProcessKillerSuite) TestKillExitStatusError() {
	ch := make(chan containerd.ExitStatus, 1)

	expectedErr := errors.New("status-err")
	ch <- *containerd.NewExitStatus(0, time.Now(), expectedErr)
	s.proc.WaitReturns(ch, nil)

	err := s.killer.Kill(context.Background(), s.proc, s.signal, s.goodEnoughTimeout)
	s.True(errors.Is(err, expectedErr))
}
