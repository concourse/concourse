package lidar

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

//go:generate counterfeiter . Limiter

type Limiter interface {
	Wait(context.Context) error
}

//go:generate counterfeiter . CheckableCounter

type CheckableCounter interface {
	CheckableCount() (int, error)
}

type CheckRateCalculator struct {
	MaxChecksPerSecond       int
	ResourceCheckingInterval time.Duration

	CheckableCounter CheckableCounter
}

// RateLimiter determines the rate at which the checks should be limited to per
// second.
//
// It is dependent on the max checks per second configuration, where if it is
// configured to -1 then it there is no limit, if it is > 0 then it is set to
// its configured value and if it is 0 (which is default) then it is calculated
// using the number of checkables and the resource checking interval.
//
// The calculated limit is determined by finding the ideal number of checks to
// run per second in order to check all the checkables in the database within
// the resource checking interval. By enforcing that ideal rate of checks, it
// will help spread out the number of checks that are started within the same
// interval.
func (c CheckRateCalculator) RateLimiter() (Limiter, error) {
	var rateOfChecks rate.Limit
	if c.MaxChecksPerSecond == -1 {
		// UNLIMITED POWER
		rateOfChecks = rate.Inf
	} else if c.MaxChecksPerSecond == 0 {
		// Fetch the number of checkables (resource config scopes) in the database
		checkableCount, err := c.CheckableCounter.CheckableCount()
		if err != nil {
			return nil, err
		}

		// Calculate the number of checks that need to be run per second in order
		// to check all the checkables within the resource checking interval
		everythingRate := float64(checkableCount) / c.ResourceCheckingInterval.Seconds()

		rateOfChecks = rate.Limit(everythingRate)
	} else {
		rateOfChecks = rate.Limit(c.MaxChecksPerSecond)
	}

	return rate.NewLimiter(rateOfChecks, 1), nil
}
