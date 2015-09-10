package worker

import (
	"net"
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

		connLog := workerLog.Session("garden-connection")

		var connection gconn.Connection

		if provider.dialer == nil {
			connection = gconn.NewWithLogger("tcp", info.Addr, connLog)
		} else {
			dialer := func(string, string) (net.Conn, error) {
				return provider.dialer("tcp", info.Addr)
			}
			connection = gconn.NewWithDialerAndLogger(dialer, connLog)
		}

		gardenConn := RetryableConnection{
			Logger:     workerLog,
			Connection: connection,
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
