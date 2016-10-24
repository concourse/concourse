package radar

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type IntervalRunner struct {
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
) *IntervalRunner {
	return &IntervalRunner{
		logger:  logger,
		clock:   clock,
		name:    name,
		scanner: scanner,
	}
}
func (r *IntervalRunner) RunFunc(signals <-chan os.Signal, ready chan<- struct{}) error {
	// do an immediate initial check
	var interval time.Duration = 0

	close(ready)

	for {
		timer := r.clock.NewTimer(interval)

		select {
		case <-signals:
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
