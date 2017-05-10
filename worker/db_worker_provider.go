package worker

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/clock"
	gclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/worker/transport"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/retryhttp"
	"github.com/cppforlife/go-semi-semantic/version"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . LockDB

type LockDB interface {
	AcquireVolumeCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
	AcquireContainerCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
}

var ErrDesiredWorkerNotRunning = errors.New("desired garden worker is not known to be running")

type ErrWorkerVersionIncompatible struct {
	HaveWorkerVersion *string
	WantWorkerVersion version.Version
}

func (err ErrWorkerVersionIncompatible) Error() string {
	if err.HaveWorkerVersion == nil {
		return fmt.Sprintf("worker version is nil (want %s)",
			err.WantWorkerVersion.String())
	}

	return fmt.Sprintf("worker version is incompatible (have %s, want %s)",
		*err.HaveWorkerVersion,
		err.WantWorkerVersion.String())
}

type dbWorkerProvider struct {
	lockDB                          LockDB
	retryBackOffFactory             retryhttp.BackOffFactory
	imageFactory                    ImageFactory
	dbResourceCacheFactory          dbng.ResourceCacheFactory
	dbResourceConfigFactory         dbng.ResourceConfigFactory
	dbWorkerBaseResourceTypeFactory dbng.WorkerBaseResourceTypeFactory
	dbVolumeFactory                 dbng.VolumeFactory
	dbTeamFactory                   dbng.TeamFactory
	dbWorkerFactory                 dbng.WorkerFactory
	workerVersion                   version.Version
}

func NewDBWorkerProvider(
	lockDB LockDB,
	retryBackOffFactory retryhttp.BackOffFactory,
	imageFactory ImageFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	dbWorkerBaseResourceTypeFactory dbng.WorkerBaseResourceTypeFactory,
	dbVolumeFactory dbng.VolumeFactory,
	dbTeamFactory dbng.TeamFactory,
	workerFactory dbng.WorkerFactory,
	workerVersion version.Version,
) WorkerProvider {
	return &dbWorkerProvider{
		lockDB:                          lockDB,
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
		if savedWorker.State() != dbng.WorkerStateRunning {
			continue
		}

		workerLog := logger.Session("running-worker")
		worker, err := provider.newGardenWorker(workerLog, tikTok, savedWorker)
		if err != nil {
			continue
		}

		workers = append(workers, worker)
	}

	return workers, nil
}

func (provider *dbWorkerProvider) GetWorker(logger lager.Logger, name string) (Worker, bool, error) {
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

	worker, err := provider.newGardenWorker(logger.Session("worker-by-name"), tikTok, savedWorker)
	return worker, found, err
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

	worker, err := provider.newGardenWorker(logger.Session("worker-for-container"), clock.NewClock(), dbWorker)
	return worker, true, err
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

	worker, err := provider.newGardenWorker(logger.Session("worker-for-build"), clock.NewClock(), dbWorker)
	return worker, true, err
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

	worker, err := provider.newGardenWorker(logger.Session("worker-for-resource-check"), clock.NewClock(), dbWorker)
	return worker, true, err
}

func (provider *dbWorkerProvider) newGardenWorker(logger lager.Logger, tikTok clock.Clock, savedWorker dbng.Worker) (Worker, error) {
	versionCheckLog := logger.Session("check-version", lager.Data{
		"want-worker-version": provider.workerVersion.String(),
		"have-worker-version": savedWorker.Version(),
	})

	compatible := provider.workerVersionIsCompatible(versionCheckLog, savedWorker.Version())
	if !compatible {
		return nil, ErrWorkerVersionIncompatible{HaveWorkerVersion: savedWorker.Version(), WantWorkerVersion: provider.workerVersion}
	}

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
	), nil
}

func (provider *dbWorkerProvider) workerVersionIsCompatible(logger lager.Logger, savedWorkerVersion *string) bool {
	if savedWorkerVersion == nil {
		logger.Info("empty-worker-version")
		return false
	}

	v, err := version.NewVersionFromString(*savedWorkerVersion)
	if err != nil {
		logger.Error("failed-to-parse-version", err)
		return false
	}

	if v.Release.Components[0].Compare(provider.workerVersion.Release.Components[0]) != 0 {
		logger.Info("different-major-version")
		return false
	}

	if v.Release.Components[1].Compare(provider.workerVersion.Release.Components[1]) == -1 {
		logger.Info("older-minor-version")
		return false
	}

	return true
}
