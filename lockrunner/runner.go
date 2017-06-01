package lockrunner

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . Task

type Task interface {
	Run() error
}

func NewRunner(
	logger lager.Logger,
	task Task,
	taskName string,
	lockFactory lock.LockFactory,
	clock clock.Clock,
	interval time.Duration,
) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)

		ticker := clock.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C():
				lockLogger := logger.Session("lock-task", lager.Data{"task-name": taskName})
				lockLogger.Debug("tick")

				lock, acquired, err := lockFactory.Acquire(lockLogger, lock.NewTaskLockID(taskName))

				if err != nil {
					lockLogger.Error("failed-to-get-lock", err)
					break
				}

				if !acquired {
					lockLogger.Debug("did-not-get-lock")
					break
				}

				lockLogger.Debug("run-task", lager.Data{"task-name": taskName})

				err = task.Run()
				if err != nil {
					lockLogger.Error("failed-to-run-task", err, lager.Data{"task-name": taskName})
				}

				lock.Release()
			case <-signals:
				return nil
			}
		}
	})
}
