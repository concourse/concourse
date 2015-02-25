package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Worker(workerInfo db.WorkerInfo) atc.Worker {
	return atc.Worker{
		Addr:             workerInfo.Addr,
		ActiveContainers: workerInfo.ActiveContainers,
		ResourceTypes:    workerInfo.ResourceTypes,
		Platform:         workerInfo.Platform,
		Tags:             workerInfo.Tags,
	}
}
