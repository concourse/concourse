package worker

import (
	"os"
	"time"

	c "code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . SaveWorkerDB

type SaveWorkerDB interface {
	SaveWorker(db.WorkerInfo, time.Duration) (db.SavedWorker, error)
}

func NewHardcoded(
	logger lager.Logger, workerDB SaveWorkerDB, clock c.Clock,
	gardenAddr string, baggageclaimURL string, resourceTypesNG []atc.WorkerResourceType,
) ifrit.RunFunc {
	return func(signals <-chan os.Signal, ready chan<- struct{}) error {
		workerInfo := db.WorkerInfo{
			GardenAddr:       gardenAddr,
			BaggageclaimURL:  baggageclaimURL,
			ActiveContainers: 0,
			ResourceTypes:    resourceTypesNG,
			Platform:         "linux",
			Tags:             []string{},
			Name:             gardenAddr,
		}

		_, err := workerDB.SaveWorker(workerInfo, 30*time.Second)
		if err != nil {
			logger.Error("could-not-save-garden-worker-provided", err)
			return err
		}

		ticker := clock.NewTicker(10 * time.Second)

		close(ready)

	dance:
		for {
			select {
			case <-ticker.C():
				_, err = workerDB.SaveWorker(workerInfo, 30*time.Second)
				if err != nil {
					logger.Error("could-not-save-garden-worker-provided", err)
				}
			case <-signals:
				ticker.Stop()
				break dance
			}
		}

		return nil
	}
}
