package lockrunner

import (
	"context"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db/lock"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . Task

type Task interface {
	Run(context.Context) error
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
				lockLogger := logger.Session("tick")

				lock, acquired, err := lockFactory.Acquire(lockLogger, lock.NewTaskLockID(taskName))
				if err != nil {
					break
				}

				if !acquired {
					lockLogger.Debug(fmt.Sprintln("failed to acquire a lock for ", taskName))
					break
				}

				ctx := lagerctx.NewContext(context.Background(), lockLogger)

				err = task.Run(ctx)
				if err != nil {
					lockLogger.Error("failed-to-run-task", err, lager.Data{"task-name": taskName})
				}

				err = lock.Release()
				if err != nil {
					lockLogger.Error("failed-to-release", err)
					break
				}
			case <-signals:
				return nil
			}
		}
	})
}
