package builds

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . BuildTracker

type BuildTracker interface {
	Track() error
	Release()
}

//go:generate counterfeiter . Notifications

type Notifications interface {
	Listen(channel string) (chan bool, error)
	Unlisten(channel string, notifier chan bool) error
	Notify(ctx context.Context, channel string) error
}

func NewRunner(
	logger lager.Logger,
	clock clock.Clock,
	tracker BuildTracker,
	interval time.Duration,
	notifications Notifications,
	lockFactory lock.LockFactory,
	componentFactory db.ComponentFactory,
) ifrit.Runner {
	return &runner{
		logger:           logger,
		clock:            clock,
		tracker:          tracker,
		interval:         interval,
		notifications:    notifications,
		componentName:    atc.ComponentBuildTracker,
		lockFactory:      lockFactory,
		componentFactory: componentFactory,
	}
}

type runner struct {
	logger           lager.Logger
	clock            clock.Clock
	tracker          BuildTracker
	interval         time.Duration
	notifications    Notifications
	componentName    string
	lockFactory      lock.LockFactory
	componentFactory db.ComponentFactory
}

func (r *runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	notifier, err := r.notifications.Listen(r.componentName)
	if err != nil {
		return err
	}

	defer r.notifications.Unlisten(r.componentName, notifier)

	ticker := r.clock.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		ctx, cancel := context.WithCancel(context.Background())

		select {
		case <-ticker.C():
			r.run(ctx, false)

		case <-notifier:
			r.run(ctx, true)

		case <-signals:
			cancel()
			r.logger.Info("releasing-tracker")
			r.tracker.Release()
			r.logger.Info("released-tracker")
			return r.notifications.Notify(ctx, r.componentName)
		}
	}
}

func (r *runner) run(ctx context.Context, force bool) error {

	lock, acquired, err := r.lockFactory.Acquire(r.logger, lock.NewTaskLockID(r.componentName))
	if err != nil {
		r.logger.Error("failed-to-acquire-lock", err)
		return err
	}

	if !acquired {
		r.logger.Debug("lock-cannot-be-acquired", lager.Data{"name": r.componentName})
		return nil
	}

	defer lock.Release()

	component, found, err := r.componentFactory.Find(r.componentName)
	if err != nil {
		r.logger.Error("failed-to-find-component", err)
		return err
	}

	if !found {
		r.logger.Info("component-not-found", lager.Data{"name": r.componentName})
		return nil
	}

	if component.Paused() {
		r.logger.Debug("component-is-paused", lager.Data{"name": r.componentName})
		return nil
	}

	if !force && !component.IntervalElapsed() {
		r.logger.Debug("component-interval-not-reached", lager.Data{"name": r.componentName, "last-ran": component.LastRan()})
		return nil
	}

	if err = r.tracker.Track(); err != nil {
		r.logger.Error("failed-to-run-task", err, lager.Data{"task-name": r.componentName})
		return err
	}

	if err = component.UpdateLastRan(); err != nil {
		r.logger.Error("failed-to-update-last-ran", err)
		return err
	}

	return nil
}
