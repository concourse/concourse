package runtime

import (
	"context"
	"time"

	"golang.org/x/sync/semaphore"
)

type TimeoutWithByPassLock struct {
	semaphore *semaphore.Weighted
	timeout   time.Duration
	enabled   bool
}

// NewTimeoutLimitLock returns a lock that allows only 1 entity to hold the lock at a time
//    Lock acquisition will block until the lock is acquired or the configured `timeout` elapses
//    Lock can be bypassed by specifying setting `enabled` to false

func NewTimeoutLimitLock(timeout time.Duration, enabled bool) TimeoutWithByPassLock {
	return TimeoutWithByPassLock{
		semaphore: semaphore.NewWeighted(1),
		timeout:   timeout,
		enabled:   enabled,
	}
}

func (tl TimeoutWithByPassLock) Acquire(ctx context.Context) error {
	if !tl.enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, tl.timeout)
	defer cancel()

	return tl.semaphore.Acquire(ctx, 1)
}

func (tl TimeoutWithByPassLock) Release() {
	if tl.enabled {
		tl.semaphore.Release(1)
	}
}
