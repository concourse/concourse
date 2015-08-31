package worker

import (
	"time"

	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . WorkerDB

type WorkerDB interface {
	Workers() ([]db.WorkerInfo, error)
}

type dbProvider struct {
	logger lager.Logger
	db     WorkerDB
	dialer gconn.DialerFunc
}

func NewDBWorkerProvider(
	logger lager.Logger,
	db WorkerDB,
	dialer gconn.DialerFunc,
) WorkerProvider {
	return &dbProvider{
		logger: logger,
		db:     db,
		dialer: dialer,
	}
}

func (provider *dbProvider) Workers() ([]Worker, error) {
	workerInfos, err := provider.db.Workers()
	if err != nil {
		return nil, err
	}

	tikTok := clock.NewClock()

	workers := make([]Worker, len(workerInfos))
	for i, info := range workerInfos {
		workerLog := provider.logger.Session("worker-connection", lager.Data{
			"addr": info.Addr,
		})

		gardenConn := RetryableConnection{
			Logger:     workerLog,
			Connection: gconn.NewWithDialerAndLogger("tcp", info.Addr, provider.dialer, workerLog.Session("garden-connection")),
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
			info.Addr,
		)
	}

	return workers, nil
}
