package runtime_test

import (
	"context"
	"time"

	"github.com/concourse/concourse/worker/runtime"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TimeoutLockSuite struct {
	suite.Suite
	*require.Assertions

	ctx context.Context
}

func (s *TimeoutLockSuite) SetupTest() {
	s.ctx = context.TODO()
}

// When disabled, can always successfully acquire & release lock
//
func (s *TimeoutLockSuite) TestLockWhenDisabled() {
	lock := runtime.NewTimeoutLimitLock(time.Millisecond*1, false)

	err := lock.Acquire(s.ctx)
	s.NoError(err)

	err = lock.Acquire(s.ctx)
	s.NoError(err)

	lock.Release()
	lock.Release()

}

// When enabled, can successfully acquire & release lock
//
func (s *TimeoutLockSuite) TestLockWithTimeout() {
	lock := runtime.NewTimeoutLimitLock(time.Millisecond*1, true)

	err := lock.Acquire(s.ctx)
	s.NoError(err)
	defer lock.Release()

}

// When enabled, acquiring fails if it timeout elapses
//
func (s *TimeoutLockSuite) TestLockWithTimeoutFailure() {
	lock := runtime.NewTimeoutLimitLock(time.Millisecond*1, true)

	err := lock.Acquire(s.ctx)
	s.NoError(err)
	defer lock.Release()

	err = lock.Acquire(s.ctx)
	s.EqualError(err, "context deadline exceeded")

}
