package lostandfound

import (
	"os"
	"time"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . RunnerDB

type RunnerDB interface {
	LeaseCacheInvalidation(interval time.Duration) (db.Lease, bool, error)
}

func NewRunner(
	logger lager.Logger,
	baggageCollector BaggageCollector,
	db RunnerDB,
	clock clock.Clock,
	interval time.Duration,
	leaseInterval time.Duration,
) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {

		close(ready)

		ticker := clock.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C():
				leaseLogger := logger.Session("lease-invalidate-cache")
				leaseLogger.Info("tick")

				lease, leased, err := db.LeaseCacheInvalidation(leaseInterval)

				if err != nil {
					leaseLogger.Error("failed-to-get-lease", err)
					break
				}

				if !leased {
					leaseLogger.Debug("did-not-get-lease")
					break
				}

				leaseLogger.Info("collecting-baggage")
				err = baggageCollector.Collect()
				if err != nil {
					leaseLogger.Error("failed-to-collect-baggage", err)
				}

				lease.Break()
			case <-signals:
				return nil
			}
		}
	})
}
