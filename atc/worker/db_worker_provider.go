package worker

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/atc/worker/transport"
	"github.com/concourse/retryhttp"
	"github.com/cppforlife/go-semi-semantic/version"

	"github.com/concourse/concourse/atc/db"
)

type dbWorkerProvider struct {
	lockFactory                       lock.LockFactory
	retryBackOffFactory               retryhttp.BackOffFactory
	resourceFetcher                   Fetcher
	imageFactory                      ImageFactory
	dbResourceCacheFactory            db.ResourceCacheFactory
	dbResourceConfigFactory           db.ResourceConfigFactory
	dbWorkerBaseResourceTypeFactory   db.WorkerBaseResourceTypeFactory
	dbTaskCacheFactory                db.TaskCacheFactory
	dbWorkerTaskCacheFactory          db.WorkerTaskCacheFactory
	dbVolumeRepository                db.VolumeRepository
	dbTeamFactory                     db.TeamFactory
	dbWorkerFactory                   db.WorkerFactory
	workerVersion                     version.Version
	baggageclaimResponseHeaderTimeout time.Duration
	gardenRequestTimeout              time.Duration
}

func NewDBWorkerProvider(
	lockFactory lock.LockFactory,
	retryBackOffFactory retryhttp.BackOffFactory,
	imageFactory ImageFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	dbWorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory,
	dbTaskCacheFactory db.TaskCacheFactory,
	dbWorkerTaskCacheFactory db.WorkerTaskCacheFactory,
	dbVolumeRepository db.VolumeRepository,
	dbTeamFactory db.TeamFactory,
	workerFactory db.WorkerFactory,
	workerVersion version.Version,
	baggageclaimResponseHeaderTimeout, gardenRequestTimeout time.Duration,
) WorkerProvider {
	return &dbWorkerProvider{
		lockFactory:                       lockFactory,
		retryBackOffFactory:               retryBackOffFactory,
		imageFactory:                      imageFactory,
		dbResourceCacheFactory:            dbResourceCacheFactory,
		dbResourceConfigFactory:           dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory:   dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory:                dbTaskCacheFactory,
		dbWorkerTaskCacheFactory:          dbWorkerTaskCacheFactory,
		dbVolumeRepository:                dbVolumeRepository,
		dbTeamFactory:                     dbTeamFactory,
		dbWorkerFactory:                   workerFactory,
		workerVersion:                     workerVersion,
		baggageclaimResponseHeaderTimeout: baggageclaimResponseHeaderTimeout,
		gardenRequestTimeout:              gardenRequestTimeout,
	}
}

func (provider *dbWorkerProvider) RunningWorkers(logger lager.Logger) ([]Worker, error) {
	savedWorkers, err := provider.dbWorkerFactory.Workers()
	if err != nil {
		return nil, err
	}

	buildContainersCountPerWorker, err := provider.dbWorkerFactory.BuildContainersCountPerWorker()
	if err != nil {
		return nil, err
	}

	workers := []Worker{}

	for _, savedWorker := range savedWorkers {
		if savedWorker.State() != db.WorkerStateRunning {
			continue
		}

		workerLog := logger.Session("running-worker")
		worker := provider.NewGardenWorker(
			workerLog,
			savedWorker,
			buildContainersCountPerWorker[savedWorker.Name()],
		)
		if !worker.IsVersionCompatible(workerLog, provider.workerVersion) {
			continue
		}

		workers = append(workers, worker)
	}

	return workers, nil
}

func (provider *dbWorkerProvider) FindWorkersForContainerByOwner(
	logger lager.Logger,
	owner db.ContainerOwner,
) ([]Worker, error) {
	logger = logger.Session("worker-for-container")
	dbWorkers, err := provider.dbWorkerFactory.FindWorkersForContainerByOwner(owner)
	if err != nil {
		return nil, err
	}

	var workers []Worker
	for _, w := range dbWorkers {
		worker := provider.NewGardenWorker(logger, w, 0)
		if worker.IsVersionCompatible(logger, provider.workerVersion) {
			workers = append(workers, worker)
		}
	}

	return workers, nil
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

	worker := provider.NewGardenWorker(logger, dbWorker, 0)
	if !worker.IsVersionCompatible(logger, provider.workerVersion) {
		return nil, false, nil
	}
	return worker, true, err
}

func (provider *dbWorkerProvider) FindWorkerForVolume(
	logger lager.Logger,
	teamID int,
	handle string,
) (Worker, bool, error) {
	logger = logger.Session("worker-for-volume")
	team := provider.dbTeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForVolume(handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	worker := provider.NewGardenWorker(logger, dbWorker, 0)
	if !worker.IsVersionCompatible(logger, provider.workerVersion) {
		return nil, false, nil
	}
	return worker, true, err
}

func (provider *dbWorkerProvider) NewGardenWorker(logger lager.Logger, savedWorker db.Worker, buildContainersCount int) Worker {
	gcf := gclient.NewGardenClientFactory(
		provider.dbWorkerFactory,
		logger.Session("garden-connection"),
		savedWorker.Name(),
		savedWorker.GardenAddr(),
		provider.retryBackOffFactory,
		provider.gardenRequestTimeout,
	)

	gClient := gcf.NewClient()

	bClient := bclient.New("", transport.NewBaggageclaimRoundTripper(
		savedWorker.Name(),
		savedWorker.BaggageclaimURL(),
		provider.dbWorkerFactory,
		&http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: provider.baggageclaimResponseHeaderTimeout,
		},
	))

	volumeClient := NewVolumeClient(
		bClient,
		savedWorker,
		clock.NewClock(),
		provider.lockFactory,
		provider.dbVolumeRepository,
		provider.dbWorkerBaseResourceTypeFactory,
		provider.dbTaskCacheFactory,
		provider.dbWorkerTaskCacheFactory,
	)

	return NewGardenWorker(
		gClient,
		provider.dbVolumeRepository,
		volumeClient,
		provider.imageFactory,
		provider.resourceFetcher,
		provider.dbTeamFactory,
		savedWorker,
		buildContainersCount,
	)
}
