package worker

import (
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

	workers := make([]Worker, len(workerInfos))
	for i, info := range workerInfos {
		workers[i] = NewGardenWorker(
			gclient.New(gconn.New("tcp", info.Addr)),
			clock.NewClock(),
			info.ActiveContainers,
			info.ResourceTypes,
		)
	}

	return workers, nil
}
