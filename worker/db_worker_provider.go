package worker

import (
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

	"github.com/concourse/atc/db"
)

type dbWorkerProvider struct {
	lockFactory                       lock.LockFactory
	retryBackOffFactory               retryhttp.BackOffFactory
	imageFactory                      ImageFactory
	dbResourceCacheFactory            db.ResourceCacheFactory
	dbResourceConfigFactory           db.ResourceConfigFactory
	dbWorkerBaseResourceTypeFactory   db.WorkerBaseResourceTypeFactory
	dbWorkerTaskCacheFactory          db.WorkerTaskCacheFactory
	dbVolumeRepository                db.VolumeRepository
	dbTeamFactory                     db.TeamFactory
	dbWorkerFactory                   db.WorkerFactory
	workerVersion                     *version.Version
	baggageclaimResponseHeaderTimeout time.Duration
}

func NewDBWorkerProvider(
	lockFactory lock.LockFactory,
	retryBackOffFactory retryhttp.BackOffFactory,
	imageFactory ImageFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	dbWorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory,
	dbWorkerTaskCacheFactory db.WorkerTaskCacheFactory,
	dbVolumeRepository db.VolumeRepository,
	dbTeamFactory db.TeamFactory,
	workerFactory db.WorkerFactory,
	workerVersion *version.Version,
	baggageclaimResponseHeaderTimeout time.Duration,
) WorkerProvider {
	return &dbWorkerProvider{
		lockFactory:                       lockFactory,
		retryBackOffFactory:               retryBackOffFactory,
		imageFactory:                      imageFactory,
		dbResourceCacheFactory:            dbResourceCacheFactory,
		dbResourceConfigFactory:           dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory:   dbWorkerBaseResourceTypeFactory,
		dbWorkerTaskCacheFactory:          dbWorkerTaskCacheFactory,
		dbVolumeRepository:                dbVolumeRepository,
		dbTeamFactory:                     dbTeamFactory,
		dbWorkerFactory:                   workerFactory,
		workerVersion:                     workerVersion,
		baggageclaimResponseHeaderTimeout: baggageclaimResponseHeaderTimeout,
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
		worker := provider.NewGardenWorker(workerLog, tikTok, savedWorker)
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

	worker := provider.NewGardenWorker(logger, clock.NewClock(), dbWorker)
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

	worker := provider.NewGardenWorker(logger, clock.NewClock(), dbWorker)
	if !worker.IsVersionCompatible(logger, provider.workerVersion) {
		return nil, false, nil
	}
	return worker, true, err
}

func (provider *dbWorkerProvider) NewGardenWorker(logger lager.Logger, tikTok clock.Clock, savedWorker db.Worker) Worker {
	gcf := NewGardenConnectionFactory(
		provider.dbWorkerFactory,
		logger.Session("garden-connection"),
		savedWorker.Name(),
		savedWorker.GardenAddr(),
		provider.retryBackOffFactory,
	)

	gClient := gclient.New(NewRetryableConnection(gcf.BuildConnection()))

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
		provider.dbWorkerTaskCacheFactory,
	)

	containerProvider := NewContainerProvider(
		gClient,
		bClient,
		// rClient,
		volumeClient,
		savedWorker,
		tikTok,
		provider.imageFactory,
		provider.dbVolumeRepository,
		provider.dbTeamFactory,
		provider.lockFactory,
	)

	return NewGardenWorker(
		gClient,
		bClient,
		// rClient,
		containerProvider,
		volumeClient,
		savedWorker,
		tikTok,
	)
}
