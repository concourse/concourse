package lockrunner

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . Task

type Task interface {
	Run(context.Context) error
}

type runner struct {
	logger           lager.Logger
	task             Task
	componentName    string
	lockFactory      lock.LockFactory
	componentFactory db.ComponentFactory
	clock            clock.Clock
	interval         time.Duration
}

func NewRunner(
	logger lager.Logger,
	task Task,
	componentName string,
	lockFactory lock.LockFactory,
	componentFactory db.ComponentFactory,
	clock clock.Clock,
	interval time.Duration,
) *runner {
	return &runner{
		logger:           logger,
		task:             task,
		componentName:    componentName,
		lockFactory:      lockFactory,
		componentFactory: componentFactory,
		clock:            clock,
		interval:         interval,
	}
}

func (r *runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	ticker := r.clock.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		ctx, cancel := context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, r.logger)

		select {
		case <-ticker.C():
			r.run(ctx, false)

		case <-signals:
			cancel()
			return nil
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err = r.task.Run(ctx); err != nil {
		r.logger.Error("failed-to-run-task", err, lager.Data{"task-name": r.componentName})
		return err
	}

	if err = component.UpdateLastRan(); err != nil {
		r.logger.Error("failed-to-update-last-ran", err)
		return err
	}

	return nil
}
