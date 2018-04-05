package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Worker(workerInfo db.Worker) atc.Worker {
	gardenAddr := ""
	if workerInfo.GardenAddr() != nil {
		gardenAddr = *workerInfo.GardenAddr()
	}
	baggageclaimURL := ""
	if workerInfo.BaggageclaimURL() != nil {
		baggageclaimURL = *workerInfo.BaggageclaimURL()
	}
	version := ""
	if workerInfo.Version() != nil {
		version = *workerInfo.Version()
	}

	return atc.Worker{
		GardenAddr:       gardenAddr,
		BaggageclaimURL:  baggageclaimURL,
		HTTPProxyURL:     workerInfo.HTTPProxyURL(),
		HTTPSProxyURL:    workerInfo.HTTPSProxyURL(),
		NoProxy:          workerInfo.NoProxy(),
		ActiveContainers: workerInfo.ActiveContainers(),
		ResourceTypes:    workerInfo.ResourceTypes(),
		Platform:         workerInfo.Platform(),
		Tags:             workerInfo.Tags(),
		Name:             workerInfo.Name(),
		Team:             workerInfo.TeamName(),
		State:            string(workerInfo.State()),
		StartTime:        workerInfo.StartTime(),
		Version:          version,
	}
}
