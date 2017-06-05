package worker

import (
	"errors"
	"net/http"
	"time"

	"code.cloudfoundry.org/clock"
	gclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/worker/transport"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/retryhttp"
	"github.com/cppforlife/go-semi-semantic/version"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var ErrDesiredWorkerNotRunning = errors.New("desired garden worker is not known to be running")

type dbWorkerProvider struct {
	lockFactory                     lock.LockFactory
	retryBackOffFactory             retryhttp.BackOffFactory
	imageFactory                    ImageFactory
	dbResourceCacheFactory          db.ResourceCacheFactory
	dbResourceConfigFactory         db.ResourceConfigFactory
	dbWorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory
	dbVolumeFactory                 db.VolumeFactory
	dbTeamFactory                   db.TeamFactory
	dbWorkerFactory                 db.WorkerFactory
	workerVersion                   *version.Version
}

func NewDBWorkerProvider(
	lockFactory lock.LockFactory,
	retryBackOffFactory retryhttp.BackOffFactory,
	imageFactory ImageFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	dbWorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory,
	dbVolumeFactory db.VolumeFactory,
	dbTeamFactory db.TeamFactory,
	workerFactory db.WorkerFactory,
	workerVersion *version.Version,
) WorkerProvider {
	return &dbWorkerProvider{
		lockFactory:                     lockFactory,
		retryBackOffFactory:             retryBackOffFactory,
		imageFactory:                    imageFactory,
		dbResourceCacheFactory:          dbResourceCacheFactory,
		dbResourceConfigFactory:         dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory: dbWorkerBaseResourceTypeFactory,
		dbVolumeFactory:                 dbVolumeFactory,
		dbTeamFactory:                   dbTeamFactory,
		dbWorkerFactory:                 workerFactory,
		workerVersion:                   workerVersion,
	}
}

func (provider *dbWorkerProvider) RunningWorkers(logger lager.Logger) ([]Worker, error) {
	savedWorkers, err := provider.dbWorkerFactory.Workers()
	if err != nil {
		return nil, err
	}

	tikTok := clock.NewClock()

	workers := []Worker{}

	for _, savedWorker := range savedWorkers {
		if savedWorker.State() != db.WorkerStateRunning {
			continue
		}

		workerLog := logger.Session("running-worker")
		worker := provider.newGardenWorker(workerLog, tikTok, savedWorker)
		if !worker.IsVersionCompatible(workerLog, provider.workerVersion) {
			continue
		}

		workers = append(workers, worker)
	}

	return workers, nil
}

func (provider *dbWorkerProvider) FindWorkerForContainerByOwner(
	logger lager.Logger,
	teamID int,
	owner db.ContainerOwner,
) (Worker, bool, error) {
	logger = logger.Session("worker-for-container")
	team := provider.dbTeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForContainerByOwner(owner)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	worker := provider.newGardenWorker(logger, clock.NewClock(), dbWorker)
	if !worker.IsVersionCompatible(logger, provider.workerVersion) {
		return nil, false, nil
	}
	return worker, true, err
}

func (provider *dbWorkerProvider) FindWorkerForContainer(
	logger lager.Logger,
	teamID int,
	handle string,
) (Worker, bool, error) {
	logger = logger.Session("worker-for-container")
	team := provider.dbTeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForContainer(handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	worker := provider.newGardenWorker(logger, clock.NewClock(), dbWorker)
	if !worker.IsVersionCompatible(logger, provider.workerVersion) {
		return nil, false, nil
	}
	return worker, true, err
}

func (provider *dbWorkerProvider) FindWorkerForBuildContainer(
	logger lager.Logger,
	teamID int,
	buildID int,
	planID atc.PlanID,
) (Worker, bool, error) {
	logger = logger.Session("worker-for-build")
	team := provider.dbTeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForBuildContainer(buildID, planID)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	worker := provider.newGardenWorker(logger, clock.NewClock(), dbWorker)
	if !worker.IsVersionCompatible(logger, provider.workerVersion) {
		return nil, false, nil
	}
	return worker, true, err
}

func (provider *dbWorkerProvider) FindWorkerForResourceCheckContainer(
	logger lager.Logger,
	teamID int,
	resourceUser db.ResourceUser,
	resourceType string,
	resourceSource atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (Worker, bool, error) {
	logger = logger.Session("worker-for-resource-check")
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

	worker := provider.newGardenWorker(logger, clock.NewClock(), dbWorker)
	if !worker.IsVersionCompatible(logger, provider.workerVersion) {
		return nil, false, nil
	}

	return worker, true, err
}

func (provider *dbWorkerProvider) newGardenWorker(logger lager.Logger, tikTok clock.Clock, savedWorker db.Worker) Worker {
	gcf := NewGardenConnectionFactory(
		provider.dbWorkerFactory,
		logger.Session("garden-connection"),
		savedWorker.Name(),
		savedWorker.GardenAddr(),
		provider.retryBackOffFactory,
	)

	connection := NewRetryableConnection(gcf.BuildConnection())

	bClient := bclient.New("", transport.NewBaggageclaimRoundTripper(
		savedWorker.Name(),
		savedWorker.BaggageclaimURL(),
		provider.dbWorkerFactory,
		&http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 10 * time.Minute,
		},
	))

	volumeClient := NewVolumeClient(
		bClient,
		provider.lockFactory,
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
		provider.lockFactory,
		savedWorker.HTTPProxyURL(),
		savedWorker.HTTPSProxyURL(),
		savedWorker.NoProxy(),
		clock.NewClock(),
	)

	return NewGardenWorker(
		containerProviderFactory,
		volumeClient,
		provider,
		tikTok,
		savedWorker.ActiveContainers(),
		savedWorker.ResourceTypes(),
		savedWorker.Platform(),
		savedWorker.Tags(),
		savedWorker.TeamID(),
		savedWorker.Name(),
		savedWorker.StartTime(),
		savedWorker.Version(),
	)
}
