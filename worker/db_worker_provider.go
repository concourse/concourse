package worker

import (
	"errors"
	"net/http"
	"time"

	"code.cloudfoundry.org/clock"
	gclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/worker/transport"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/retryhttp"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . LockDB

type LockDB interface {
	AcquireVolumeCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
	AcquireContainerCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
}

var ErrDesiredWorkerNotRunning = errors.New("desired-garden-worker-is-not-known-to-be-running")

type dbWorkerProvider struct {
	logger                          lager.Logger
	lockDB                          LockDB
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
	lockDB LockDB,
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
		lockDB:                          lockDB,
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

func (provider *dbWorkerProvider) FindWorkerForContainer(
	logger lager.Logger,
	teamID int,
	handle string,
) (Worker, bool, error) {
	team := provider.dbTeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForContainer(handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return provider.newGardenWorker(clock.NewClock(), dbWorker), true, nil
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
		&http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 1 * time.Minute,
		},
	))

	volumeClient := NewVolumeClient(
		bClient,
		provider.lockDB,
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
		provider.lockDB,
		savedWorker.HTTPProxyURL(),
		savedWorker.HTTPSProxyURL(),
		savedWorker.NoProxy(),
		savedWorker.CertificatesPath(),
		savedWorker.CertificatesSymlinkedPaths(),
		clock.NewClock(),
	)

	return NewGardenWorker(
		containerProviderFactory,
		volumeClient,
		provider.lockDB,
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
