package scheduler

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

//go:generate counterfeiter . Runner

type Runner interface {
	Run(context.Context) error
}

func NewIntervalRunner(
	logger lager.Logger,
	clock clock.Clock,
	lockFactory lock.LockFactory,
	componentFactory db.ComponentFactory,
	runner Runner,
	interval time.Duration,
) ifrit.Runner {
	return &intervalRunner{
		logger:           logger,
		clock:            clock,
		componentName:    atc.ComponentScheduler,
		lockFactory:      lockFactory,
		componentFactory: componentFactory,
		runner:           runner,
		interval:         interval,
	}
}

type intervalRunner struct {
	logger           lager.Logger
	runner           Runner
	componentName    string
	lockFactory      lock.LockFactory
	componentFactory db.ComponentFactory
	clock            clock.Clock
	interval         time.Duration
}

func (r *intervalRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	r.logger.Info("start", lager.Data{
		"interval": r.interval.String(),
	})
	defer r.logger.Info("done")

	ticker := r.clock.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		ctx, cancel := context.WithCancel(context.Background())

		select {
		case <-ticker.C():
			r.run(ctx, false)

		case <-signals:
			cancel()
			return nil
		}
	}
}

func (r *intervalRunner) run(ctx context.Context, force bool) error {
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

	if err = r.runner.Run(ctx); err != nil {
		r.logger.Error("failed-to-run-task", err, lager.Data{"task-name": r.componentName})
		return err
	}

	if err = component.UpdateLastRan(); err != nil {
		r.logger.Error("failed-to-update-last-ran", err)
		return err
	}

	return nil
}
