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
	Workers() ([]db.SavedWorker, error)
	GetWorker(string) (db.SavedWorker, bool, error)
	CreateContainer(db.Container, time.Duration) (db.SavedContainer, error)
	GetContainer(string) (db.SavedContainer, bool, error)
	FindContainerByIdentifier(db.ContainerIdentifier) (db.SavedContainer, bool, error)

	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error
	ReapContainer(handle string) error

	InsertVolume(db.Volume) error
	GetVolumesByIdentifier(db.VolumeIdentifier) ([]db.SavedVolume, error)
	GetVolumeTTL(volumeHandle string) (time.Duration, bool, error)
	ReapVolume(handle string) error
	SetVolumeTTL(string, time.Duration) error
}

var ErrMultipleWorkersWithName = errors.New("More than one worker has given worker name")

type dbProvider struct {
	logger       lager.Logger
	db           WorkerDB
	dialer       gconn.DialerFunc
	retryPolicy  RetryPolicy
	imageFetcher ImageFetcher
}

func NewDBWorkerProvider(
	logger lager.Logger,
	db WorkerDB,
	dialer gconn.DialerFunc,
	retryPolicy RetryPolicy,
	imageFetcher ImageFetcher,
) WorkerProvider {
	return &dbProvider{
		logger:       logger,
		db:           db,
		dialer:       dialer,
		retryPolicy:  retryPolicy,
		imageFetcher: imageFetcher,
	}
}

func (provider *dbProvider) Workers() ([]Worker, error) {
	savedWorkers, err := provider.db.Workers()
	if err != nil {
		return nil, err
	}

	tikTok := clock.NewClock()

	workers := make([]Worker, len(savedWorkers))

	for i, savedWorker := range savedWorkers {
		workers[i] = provider.newGardenWorker(tikTok, savedWorker)
	}

	return workers, nil
}

func (provider *dbProvider) GetWorker(name string) (Worker, bool, error) {
	savedWorker, found, err := provider.db.GetWorker(name)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	tikTok := clock.NewClock()

	worker := provider.newGardenWorker(tikTok, savedWorker)

	return worker, found, nil
}

func (provider *dbProvider) FindContainerForIdentifier(id Identifier) (db.SavedContainer, bool, error) {
	return provider.db.FindContainerByIdentifier(db.ContainerIdentifier(id))
}

func (provider *dbProvider) GetContainer(handle string) (db.SavedContainer, bool, error) {
	return provider.db.GetContainer(handle)
}

func (provider *dbProvider) ReapContainer(handle string) error {
	return provider.db.ReapContainer(handle)
}

func (provider *dbProvider) newGardenWorker(tikTok clock.Clock, savedWorker db.SavedWorker) Worker {
	workerLog := provider.logger.Session("worker-connection", lager.Data{
		"addr": savedWorker.GardenAddr,
	})

	gardenConn := NewRetryableConnection(
		workerLog,
		tikTok,
		provider.retryPolicy,
		NewGardenConnectionFactory(
			provider.db,
			provider.dialer,
			provider.logger.Session("garden-connection"),
			savedWorker.Name,
			savedWorker.GardenAddr,
		),
	)

	var bClient baggageclaim.Client
	if savedWorker.BaggageclaimURL != "" {
		bClient = bclient.New(savedWorker.BaggageclaimURL)
	}

	volumeFactory := NewVolumeFactory(
		provider.db,
		tikTok,
	)

	return NewGardenWorker(
		gclient.New(gardenConn),
		bClient,
		volumeFactory,
		provider.imageFetcher,
		provider.db,
		provider,
		tikTok,
		savedWorker.ActiveContainers,
		savedWorker.ResourceTypes,
		savedWorker.Platform,
		savedWorker.Tags,
		savedWorker.Name,
		savedWorker.HTTPProxyURL,
		savedWorker.HTTPSProxyURL,
		savedWorker.NoProxy,
	)
}
