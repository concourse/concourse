package radar

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . IntervalRunner
type IntervalRunner interface {
	Run(context.Context) error
}

type intervalRunner struct {
	logger        lager.Logger
	clock         clock.Clock
	id            int
	scanner       Scanner
	notifications Notifications
}

func NewIntervalRunner(
	logger lager.Logger,
	clock clock.Clock,
	id int,
	scanner Scanner,
	notifications Notifications,
) IntervalRunner {
	return &intervalRunner{
		logger:        logger,
		clock:         clock,
		id:            id,
		scanner:       scanner,
		notifications: notifications,
	}
}

func (r *intervalRunner) Run(ctx context.Context) error {

	interval := time.Duration(0)
	channel := fmt.Sprintf("resource_scan_%d", r.id)

	notifier, err := r.notifications.Listen(channel)
	if err != nil {
		return err
	}

	defer r.notifications.Unlisten(channel, notifier)

	for {
		timer := r.clock.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-notifier:
			if err = r.scanner.Scan(r.logger, r.id); err != nil {
				if err == ErrFailedToAcquireLock {
					break
				}
				return err
			}
		case <-timer.C():
			interval, err = r.scanner.Run(r.logger, r.id)
			if err != nil {
				if err == ErrFailedToAcquireLock {
					break
				}
				return err
			}
		}
	}
}
