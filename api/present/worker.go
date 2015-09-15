package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Worker(workerInfo db.WorkerInfo) atc.Worker {
	return atc.Worker{
		GardenAddr:       workerInfo.GardenAddr,
		BaggageclaimURL:  workerInfo.BaggageclaimURL,
		ActiveContainers: workerInfo.ActiveContainers,
		ResourceTypes:    workerInfo.ResourceTypes,
		Platform:         workerInfo.Platform,
		Tags:             workerInfo.Tags,
		Name:             workerInfo.Name,
	}
}
