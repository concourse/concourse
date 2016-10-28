package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

func Worker(workerInfo dbng.Worker) atc.Worker {
	return atc.Worker{
		GardenAddr:       *workerInfo.GardenAddr,
		BaggageclaimURL:  workerInfo.BaggageclaimURL,
		HTTPProxyURL:     workerInfo.HTTPProxyURL,
		HTTPSProxyURL:    workerInfo.HTTPSProxyURL,
		NoProxy:          workerInfo.NoProxy,
		ActiveContainers: workerInfo.ActiveContainers,
		ResourceTypes:    workerInfo.ResourceTypes,
		Platform:         workerInfo.Platform,
		Tags:             workerInfo.Tags,
		Name:             workerInfo.Name,
		Team:             workerInfo.TeamName,
		State:            string(workerInfo.State),
	}
}
