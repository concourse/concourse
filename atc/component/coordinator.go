package component

import (
	"code.cloudfoundry.org/lager"
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db/lock"
)

// Coordinator ensures that the given component is not executed concurrently.
type Coordinator struct {
	Locker    lock.LockFactory
	Component Component
	Runnable  Runnable
}

func (coordinator *Coordinator) RunPeriodically(ctx context.Context) bool {
	return coordinator.run(ctx, false)
}

func (coordinator *Coordinator) RunImmediately(ctx context.Context) {
	coordinator.run(ctx, true)
}

func (coordinator *Coordinator) run(ctx context.Context, immediate bool) bool {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{"name": coordinator.Component.Name()})

	lockID := lock.NewTaskLockID(coordinator.Component.Name())

	lock, acquired, err := coordinator.Locker.Acquire(logger, lockID)
	if err != nil {
		logger.Error("failed-to-acquire-lock", err)
		return false
	}

	if !acquired {
		logger.Debug("lock-unavailable")
		return false
	}

	defer lock.Release()

	exists, err := coordinator.Component.Reload()
	if err != nil {
		logger.Error("failed-to-reload-component", err)
		return false
	}

	if !exists {
		logger.Info("component-disappeared")
		return false
	}

	if coordinator.Component.Paused() {
		logger.Debug("component-paused")
		return false
	}

	if !immediate && !coordinator.Component.IntervalElapsed() {
		logger.Debug("interval-not-elapsed")
		return false
	}

	if err := coordinator.Runnable.Run(ctx); err != nil {
		logger.Error("component-failed", err)
		return true
	}

	if err := coordinator.Component.UpdateLastRan(); err != nil {
		logger.Error("failed-to-update-last-ran", err)
		return true
	}

	return true
}
