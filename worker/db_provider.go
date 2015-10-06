package worker

import (
	"errors"
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
	ReapContainer(handle string) error
}

var ErrMultipleWorkersWithName = errors.New("More than one worker has given worker name")

type dbProvider struct {
	logger      lager.Logger
	db          WorkerDB
	dialer      gconn.DialerFunc
	retryPolicy RetryPolicy
}

func NewDBWorkerProvider(
	logger lager.Logger,
	db WorkerDB,
	dialer gconn.DialerFunc,
	retryPolicy RetryPolicy,
) WorkerProvider {
	return &dbProvider{
		logger:      logger,
		db:          db,
		dialer:      dialer,
		retryPolicy: retryPolicy,
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
		workers[i] = provider.newGardenWorker(tikTok, info)
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

	worker := provider.newGardenWorker(tikTok, workerInfo)

	return worker, found, nil
}

func (provider *dbProvider) FindContainerInfoForIdentifier(id Identifier) (db.ContainerInfo, bool, error) {
	return provider.db.FindContainerInfoByIdentifier(db.ContainerIdentifier(id))
}

func (provider *dbProvider) GetContainerInfo(handle string) (db.ContainerInfo, bool, error) {
	return provider.db.GetContainerInfo(handle)
}

func (provider *dbProvider) ReapContainer(handle string) error {
	return provider.db.ReapContainer(handle)
}

func (provider *dbProvider) newGardenWorker(tikTok clock.Clock, info db.WorkerInfo) Worker {
	workerLog := provider.logger.Session("worker-connection", lager.Data{
		"addr": info.GardenAddr,
	})

	gardenConn := NewRetryableConnection(
		workerLog,
		tikTok,
		provider.retryPolicy,
		NewGardenConnectionFactory(
			provider.db,
			provider.dialer,
			provider.logger.Session("garden-connection"),
			info.Name,
			info.GardenAddr,
		),
	)

	var bClient baggageclaim.Client
	if info.BaggageclaimURL != "" {
		bClient = bclient.New(info.BaggageclaimURL)
	}

	return NewGardenWorker(
		gclient.New(gardenConn),
		bClient,
		provider.db,
		provider,
		tikTok,
		info.ActiveContainers,
		info.ResourceTypes,
		info.Platform,
		info.Tags,
		info.Name,
	)
}
