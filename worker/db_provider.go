package worker

import "github.com/concourse/atc/db"

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
		workers[i] = NewGardenWorker(info.Addr, info.ActiveContainers)
	}

	return workers, nil
}
