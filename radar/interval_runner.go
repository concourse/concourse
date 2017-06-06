package radar

import (
	"context"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . IntervalRunner
type IntervalRunner interface {
	Run(context.Context) error
}

type intervalRunner struct {
	logger  lager.Logger
	clock   clock.Clock
	name    string
	scanner Scanner
}

func NewIntervalRunner(
	logger lager.Logger,
	clock clock.Clock,
	name string,
	scanner Scanner,
) IntervalRunner {
	return &intervalRunner{
		logger:  logger,
		clock:   clock,
		name:    name,
		scanner: scanner,
	}
}

func (r *intervalRunner) Run(ctx context.Context) error {
	// do an immediate initial check
	var interval time.Duration = 0

	for {
		timer := r.clock.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C():
			var err error
			interval, err = r.scanner.Run(r.logger, r.name)
			if err != nil {
				if err == ErrFailedToAcquireLock {
					break
				}
				return err
			}
		}
	}
}
