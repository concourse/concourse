package leaserunner

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . RunnerDB

type RunnerDB interface {
	GetLease(logger lager.Logger, leaseName string, interval time.Duration) (db.Lease, bool, error)
}

//go:generate counterfeiter . Task

type Task interface {
	Run() error
}

func NewRunner(
	logger lager.Logger,
	task Task,
	taskName string,
	db RunnerDB,
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
				leaseLogger := logger.Session("lease-task", lager.Data{"task-name": taskName})
				leaseLogger.Info("tick")

				lease, leased, err := db.GetLease(leaseLogger, taskName, interval)

				if err != nil {
					leaseLogger.Error("failed-to-get-lease", err)
					break
				}

				if !leased {
					leaseLogger.Debug("did-not-get-lease")
					break
				}

				leaseLogger.Info("run-task", lager.Data{"task-name": taskName})
				err = task.Run()
				if err != nil {
					leaseLogger.Error("failed-to-run-task", err, lager.Data{"task-name": taskName})
				}

				lease.Break()
			case <-signals:
				return nil
			}
		}
	})
}
