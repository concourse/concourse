package worker

import (
	"errors"
	"net/http"
	"time"

	"code.cloudfoundry.org/clock"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/worker/transport"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/retryhttp"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . WorkerDB

type WorkerDB interface {
	PutTheRestOfThisCrapInTheDatabaseButPleaseRemoveMeLater(container db.Container, maxLifetime time.Duration) error
	GetContainer(string) (db.SavedContainer, bool, error)
	FindContainerByIdentifier(db.ContainerIdentifier) (db.SavedContainer, bool, error)
	GetPipelineByID(pipelineID int) (db.SavedPipeline, error)
	AcquireVolumeCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
	AcquireContainerCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
}

var ErrDesiredWorkerNotRunning = errors.New("desired-garden-worker-is-not-known-to-be-running")

type dbWorkerProvider struct {
	logger                          lager.Logger
	db                              WorkerDB
	dialer                          gconn.DialerFunc
	retryBackOffFactory             retryhttp.BackOffFactory
	imageFactory                    ImageFactory
	dbResourceCacheFactory          dbng.ResourceCacheFactory
	dbResourceConfigFactory         dbng.ResourceConfigFactory
	dbWorkerBaseResourceTypeFactory dbng.WorkerBaseResourceTypeFactory
	dbVolumeFactory                 dbng.VolumeFactory
	dbTeamFactory                   dbng.TeamFactory
	dbWorkerFactory                 dbng.WorkerFactory
}

func NewDBWorkerProvider(
	logger lager.Logger,
	db WorkerDB,
	dialer gconn.DialerFunc,
	retryBackOffFactory retryhttp.BackOffFactory,
	imageFactory ImageFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	dbWorkerBaseResourceTypeFactory dbng.WorkerBaseResourceTypeFactory,
	dbVolumeFactory dbng.VolumeFactory,
	dbTeamFactory dbng.TeamFactory,
	workerFactory dbng.WorkerFactory,
) WorkerProvider {
	return &dbWorkerProvider{
		logger:                          logger,
		db:                              db,
		dialer:                          dialer,
		retryBackOffFactory:             retryBackOffFactory,
		imageFactory:                    imageFactory,
		dbResourceCacheFactory:          dbResourceCacheFactory,
		dbResourceConfigFactory:         dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory: dbWorkerBaseResourceTypeFactory,
		dbVolumeFactory:                 dbVolumeFactory,
		dbTeamFactory:                   dbTeamFactory,
		dbWorkerFactory:                 workerFactory,
	}
}

func (provider *dbWorkerProvider) RunningWorkers() ([]Worker, error) {
	savedWorkers, err := provider.dbWorkerFactory.Workers()
	if err != nil {
		return nil, err
	}

	tikTok := clock.NewClock()

	workers := []Worker{}

	for _, savedWorker := range savedWorkers {
		if savedWorker.State() == dbng.WorkerStateRunning {
			workers = append(workers, provider.newGardenWorker(tikTok, savedWorker))
		}
	}

	return workers, nil
}

func (provider *dbWorkerProvider) GetWorker(name string) (Worker, bool, error) {
	savedWorker, found, err := provider.dbWorkerFactory.GetWorker(name)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	if savedWorker.State() == dbng.WorkerStateStalled ||
		savedWorker.State() == dbng.WorkerStateLanded {
		return nil, false, ErrDesiredWorkerNotRunning
	}

	tikTok := clock.NewClock()

	worker := provider.newGardenWorker(tikTok, savedWorker)

	return worker, found, nil
}

func (provider *dbWorkerProvider) FindContainerForIdentifier(id Identifier) (db.SavedContainer, bool, error) {
	container, found, err := provider.db.FindContainerByIdentifier(db.ContainerIdentifier(id))
	if err != nil {
		provider.logger.Error("failed-to-find-container-by-identifier", err, lager.Data{"id": id})
	}

	return container, found, err
}

func (provider *dbWorkerProvider) FindWorkerForBuildContainer(
	logger lager.Logger,
	teamID int,
	buildID int,
	planID atc.PlanID,
) (Worker, bool, error) {
	team := provider.dbTeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForBuildContainer(buildID, planID)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return provider.newGardenWorker(clock.NewClock(), dbWorker), true, nil
}

func (provider *dbWorkerProvider) FindWorkerForResourceCheckContainer(
	logger lager.Logger,
	teamID int,
	resourceUser dbng.ResourceUser,
	resourceType string,
	resourceSource atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (Worker, bool, error) {
	team := provider.dbTeamFactory.GetByID(teamID)

	config, err := provider.dbResourceConfigFactory.FindOrCreateResourceConfig(logger, resourceUser, resourceType, resourceSource, resourceTypes)
	if err != nil {
		return nil, false, err
	}

	dbWorker, found, err := team.FindWorkerForResourceCheckContainer(config)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return provider.newGardenWorker(clock.NewClock(), dbWorker), true, nil
}

func (provider *dbWorkerProvider) GetContainer(handle string) (db.SavedContainer, bool, error) {
	return provider.db.GetContainer(handle)
}

func (provider *dbWorkerProvider) newGardenWorker(tikTok clock.Clock, savedWorker dbng.Worker) Worker {
	gcf := NewGardenConnectionFactory(
		provider.dbWorkerFactory,
		provider.logger.Session("garden-connection"),
		savedWorker.Name(),
		savedWorker.GardenAddr(),
		provider.retryBackOffFactory,
	)

	connection := NewRetryableConnection(gcf.BuildConnection())

	bClient := bclient.New("", transport.NewBaggageclaimRoundTripper(
		savedWorker.Name(),
		savedWorker.BaggageclaimURL(),
		provider.dbWorkerFactory,
		&http.Transport{DisableKeepAlives: true},
	))

	volumeClient := NewVolumeClient(
		bClient,
		provider.db,
		provider.dbVolumeFactory,
		provider.dbWorkerBaseResourceTypeFactory,
		clock.NewClock(),
		savedWorker,
	)

	containerProviderFactory := NewContainerProviderFactory(
		gclient.New(connection),
		bClient,
		volumeClient,
		provider.imageFactory,
		provider.dbVolumeFactory,
		provider.dbResourceCacheFactory,
		provider.dbResourceConfigFactory,
		provider.dbTeamFactory,
		provider.db,
		savedWorker.HTTPProxyURL(),
		savedWorker.HTTPSProxyURL(),
		savedWorker.NoProxy(),
		clock.NewClock(),
	)

	return NewGardenWorker(
		containerProviderFactory,
		volumeClient,
		provider.db,
		provider,
		tikTok,
		savedWorker.ActiveContainers(),
		savedWorker.ResourceTypes(),
		savedWorker.Platform(),
		savedWorker.Tags(),
		savedWorker.TeamID(),
		savedWorker.Name(),
		savedWorker.StartTime(),
	)
}
