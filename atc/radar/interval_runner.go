package radar

import (
	"context"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . IntervalRunner
type IntervalRunner interface {
	Run(context.Context) error
}

//go:generate counterfeiter . ResourceNotifier
type ResourceNotifier interface {
	ID() int
	ScanNotifier() (db.Notifier, error)
}

type intervalRunner struct {
	logger   lager.Logger
	clock    clock.Clock
	notifier ResourceNotifier
	scanner  Scanner
}

func NewIntervalRunner(
	logger lager.Logger,
	clock clock.Clock,
	notifier ResourceNotifier,
	scanner Scanner,
) IntervalRunner {
	return &intervalRunner{
		logger:   logger,
		clock:    clock,
		notifier: notifier,
		scanner:  scanner,
	}
}

func (r *intervalRunner) Run(ctx context.Context) error {
	interval := time.Duration(0)

	notifier, err := r.notifier.ScanNotifier()
	if err != nil {
		return err
	}

	defer notifier.Close()

	for {
		timer := r.clock.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-notifier.Notify():
			if err = r.scanner.Scan(r.logger, r.notifier.ID()); err != nil {
				if err == ErrFailedToAcquireLock {
					break
				}
				return err
			}
		case <-timer.C():
			interval, err = r.scanner.Run(r.logger, r.notifier.ID())
			if err != nil {
				if err == ErrFailedToAcquireLock {
					break
				}
				return err
			}
		}
	}
}
