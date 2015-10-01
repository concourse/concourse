package worker

import (
	"errors"
	"net"
	"time"

	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/concourse/baggageclaim"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . WorkerDB

type WorkerDB interface {
	Workers() ([]db.WorkerInfo, error)
	GetWorker(string) (db.WorkerInfo, bool, error)
	CreateContainerInfo(db.ContainerInfo, time.Duration) error
	GetContainerInfo(string) (db.ContainerInfo, bool, error)
	FindContainerInfoByIdentifier(db.ContainerIdentifier) (db.ContainerInfo, bool, error)

	UpdateExpiresAtOnContainerInfo(handle string, ttl time.Duration) error
}

var ErrMultipleWorkersWithName = errors.New("More than one worker has given worker name")

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
		// this is very important (to prevent closures capturing last value)
		addr := info.GardenAddr

		workers[i] = provider.newGardenWorker(addr, tikTok, info)
	}

	return workers, nil
}

func (provider *dbProvider) GetWorker(name string) (Worker, bool, error) {
	workerInfo, found, err := provider.db.GetWorker(name)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	tikTok := clock.NewClock()

	worker := provider.newGardenWorker(workerInfo.GardenAddr, tikTok, workerInfo)

	return worker, found, nil
}

func (provider *dbProvider) FindContainerInfoForIdentifier(id Identifier) (db.ContainerInfo, bool, error) {
	containerIdentifier := db.ContainerIdentifier{
		Name:         id.Name,
		PipelineName: id.PipelineName,
		BuildID:      id.BuildID,
		Type:         id.Type,
		WorkerName:   id.WorkerName,
		CheckType:    id.CheckType,
		CheckSource:  id.CheckSource,
	}
	return provider.db.FindContainerInfoByIdentifier(containerIdentifier)
}

func (provider *dbProvider) GetContainerInfo(handle string) (db.ContainerInfo, bool, error) {
	return provider.db.GetContainerInfo(handle)
}

func (provider *dbProvider) newGardenWorker(addr string, tikTok clock.Clock, info db.WorkerInfo) Worker {
	workerLog := provider.logger.Session("worker-connection", lager.Data{
		"addr": addr,
	})

	connLog := workerLog.Session("garden-connection")

	var connection gconn.Connection

	if provider.dialer == nil {
		connection = gconn.NewWithLogger("tcp", addr, connLog)
	} else {
		dialer := func(string, string) (net.Conn, error) {
			return provider.dialer("tcp", addr)
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

	var bClient baggageclaim.Client
	if info.BaggageclaimURL != "" {
		bClient = bclient.New(info.BaggageclaimURL)
	}

	return NewGardenWorker(
		gclient.New(gardenConn),
		bClient,
		provider.db,
		tikTok,
		info.ActiveContainers,
		info.ResourceTypes,
		info.Platform,
		info.Tags,
		info.GardenAddr,
	)
}
