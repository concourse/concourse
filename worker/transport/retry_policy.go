package transport

import (
	"math"
	"time"
)

type ExponentialRetryPolicy struct {
	Timeout time.Duration
}

const maxRetryDelay = 16 * time.Second

func (policy ExponentialRetryPolicy) DelayFor(attempts uint) (time.Duration, bool) {
	// buckle up
	attemptsBackingOff := uint(math.Log2(maxRetryDelay.Seconds()))
	timeSpentMaxedOut := policy.Timeout - (1<<(attemptsBackingOff+1))*time.Second
	attemptsMaxedOut := uint(float64(timeSpentMaxedOut) / float64(maxRetryDelay))

	maxAttempts := attemptsMaxedOut + attemptsBackingOff
	if attempts > maxAttempts {
		return 0, false
	}

	exponentialDelay := (1 << (attempts - 1)) * time.Second
	if exponentialDelay > maxRetryDelay {
		return maxRetryDelay, true
	}

	return exponentialDelay, true
}
