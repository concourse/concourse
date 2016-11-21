package worker

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/retryhttp"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . WorkerDB

type WorkerDB interface {
	Workers() ([]db.SavedWorker, error)
	GetWorker(string) (db.SavedWorker, bool, error)
	CreateContainer(container db.Container, ttl time.Duration, maxLifetime time.Duration, volumeHandles []string) (db.SavedContainer, error)
	UpdateContainerTTLToBeRemoved(container db.Container, ttl time.Duration, maxLifetime time.Duration) (db.SavedContainer, error)
	GetContainer(string) (db.SavedContainer, bool, error)
	FindContainerByIdentifier(db.ContainerIdentifier) (db.SavedContainer, bool, error)
	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error
	ReapContainer(handle string) error
	GetPipelineByID(pipelineID int) (db.SavedPipeline, error)
	ReapVolume(handle string) error
	AcquireVolumeCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
}

var ErrDesiredWorkerNotRunning = errors.New("desired-garden-worker-is-not-known-to-be-running")

type dbProvider struct {
	logger                    lager.Logger
	db                        WorkerDB
	dialer                    gconn.DialerFunc
	retryBackOffFactory       retryhttp.BackOffFactory
	imageFactory              ImageFactory
	dbContainerFactory        DBContainerFactory
	dbResourceCacheFactory    dbng.ResourceCacheFactory
	dbResourceTypeFactory     dbng.ResourceTypeFactory
	dbResourceConfigFactory   dbng.ResourceConfigFactory
	dbBaseResourceTypeFactory dbng.BaseResourceTypeFactory
	dbVolumeFactory           dbng.VolumeFactory
	pipelineDBFactory         db.PipelineDBFactory
	dbWorkerFactory           dbng.WorkerFactory
}

func NewDBWorkerProvider(
	logger lager.Logger,
	db WorkerDB,
	dialer gconn.DialerFunc,
	retryBackOffFactory retryhttp.BackOffFactory,
	imageFactory ImageFactory,
	dbContainerFactory DBContainerFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	dbBaseResourceTypeFactory dbng.BaseResourceTypeFactory,
	dbVolumeFactory dbng.VolumeFactory,
	pipelineDBFactory db.PipelineDBFactory,
	workerFactory dbng.WorkerFactory,
) WorkerProvider {
	return &dbProvider{
		logger:                    logger,
		db:                        db,
		dialer:                    dialer,
		retryBackOffFactory:       retryBackOffFactory,
		imageFactory:              imageFactory,
		dbContainerFactory:        dbContainerFactory,
		dbResourceCacheFactory:    dbResourceCacheFactory,
		dbResourceConfigFactory:   dbResourceConfigFactory,
		dbBaseResourceTypeFactory: dbBaseResourceTypeFactory,
		dbVolumeFactory:           dbVolumeFactory,
		dbWorkerFactory:           workerFactory,
		pipelineDBFactory:         pipelineDBFactory,
	}
}

func (provider *dbProvider) Workers() ([]Worker, error) {
	savedWorkers, err := provider.dbWorkerFactory.Workers()
	if err != nil {
		return nil, err
	}

	tikTok := clock.NewClock()

	workers := []Worker{}

	for _, savedWorker := range savedWorkers {
		if savedWorker.State == dbng.WorkerStateRunning {
			workers = append(workers, provider.newGardenWorker(tikTok, savedWorker))
		}
	}

	return workers, nil
}

func (provider *dbProvider) GetWorker(name string) (Worker, bool, error) {
	savedWorker, found, err := provider.dbWorkerFactory.GetWorker(name)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	if savedWorker.State != dbng.WorkerStateRunning {
		return nil, true, ErrDesiredWorkerNotRunning
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

func (provider *dbProvider) newGardenWorker(tikTok clock.Clock, savedWorker *dbng.Worker) Worker {
	gcf := NewGardenConnectionFactory(
		provider.dbWorkerFactory,
		provider.logger.Session("garden-connection"),
		savedWorker.Name,
		*(savedWorker.GardenAddr),
		provider.retryBackOffFactory,
	)

	connection := NewRetryableConnection(gcf.BuildConnection())

	var bClient baggageclaim.Client
	if savedWorker.BaggageclaimURL != "" {
		bClient = bclient.New(savedWorker.BaggageclaimURL)
	}

	volumeClient := NewVolumeClient(
		bClient,
		provider.db,
		provider.dbVolumeFactory,
		provider.dbBaseResourceTypeFactory,
		clock.NewClock(),
		&dbng.Worker{
			Name:       savedWorker.Name,
			GardenAddr: savedWorker.GardenAddr,
		},
	)

	return NewGardenWorker(
		gclient.New(connection),
		bClient,
		volumeClient,
		provider.imageFactory,
		provider.pipelineDBFactory,
		provider.dbContainerFactory,
		provider.dbVolumeFactory,
		provider.dbResourceCacheFactory,
		provider.dbResourceConfigFactory,
		provider.db,
		provider,
		tikTok,
		savedWorker.ActiveContainers,
		savedWorker.ResourceTypes,
		savedWorker.Platform,
		savedWorker.Tags,
		savedWorker.TeamID,
		savedWorker.Name,
		*savedWorker.GardenAddr,
		savedWorker.StartTime,
		savedWorker.HTTPProxyURL,
		savedWorker.HTTPSProxyURL,
		savedWorker.NoProxy,
	)
}
