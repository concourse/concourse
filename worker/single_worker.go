package worker

import (
	"time"

	c "github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . SaveWorkerDB

type SaveWorkerDB interface {
	SaveWorker(db.WorkerInfo, time.Duration) error
}

func RegisterSingleWorker(
	logger lager.Logger, workerDB SaveWorkerDB, clock c.Clock,
	gardenAddr string, baggageclaimURL string, resourceTypesNG []atc.WorkerResourceType,
) {
	workerInfo := db.WorkerInfo{
		GardenAddr:       gardenAddr,
		BaggageclaimURL:  baggageclaimURL,
		ActiveContainers: 0,
		ResourceTypes:    resourceTypesNG,
		Platform:         "linux",
		Tags:             []string{},
		Name:             gardenAddr,
	}

	err := workerDB.SaveWorker(workerInfo, 30*time.Second)
	if err != nil {
		logger.Fatal("could-not-save-garden-worker-provided", err)
	}

	ticker := clock.NewTicker(10 * time.Second)

	go func() {
		for {
			<-ticker.C()
			err = workerDB.SaveWorker(workerInfo, 30*time.Second)
			if err != nil {
				logger.Error("could-not-save-garden-worker-provided", err)
			}
		}
	}()
}
