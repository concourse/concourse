package worker

import (
	"time"

	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/pivotal-golang/clock"

	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . WorkerDB

type WorkerDB interface {
	Workers() ([]db.WorkerInfo, error)
}

type dbProvider struct {
	db WorkerDB
}

func NewDBWorkerProvider(db WorkerDB) WorkerProvider {
	return &dbProvider{db}
}

func (provider *dbProvider) Workers() ([]Worker, error) {
	workerInfos, err := provider.db.Workers()
	if err != nil {
		return nil, err
	}

	tikTok := clock.NewClock()

	workers := make([]Worker, len(workerInfos))
	for i, info := range workerInfos {
		gardenConn := RetryableConnection{
			Connection: gconn.New("tcp", info.Addr),
			Sleeper:    tikTok,
			RetryPolicy: ExponentialRetryPolicy{
				Timeout: 5 * time.Minute,
			},
		}

		workers[i] = NewGardenWorker(
			gclient.New(gardenConn),
			tikTok,
			info.ActiveContainers,
			info.ResourceTypes,
			info.Platform,
			info.Tags,
		)
	}

	return workers, nil
}
