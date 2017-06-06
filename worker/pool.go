package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	RunningWorkers(lager.Logger) ([]Worker, error)

	FindWorkerForContainer(
		logger lager.Logger,
		teamID int,
		handle string,
	) (Worker, bool, error)

	FindWorkerForContainerByOwner(
		logger lager.Logger,
		teamID int,
		owner db.ContainerOwner,
	) (Worker, bool, error)

	FindWorkerForResourceCheckContainer(
		logger lager.Logger,
		teamID int,
		resourceUser db.ResourceUser,
		resourceType string,
		resourceSource atc.Source,
		types atc.VersionedResourceTypes,
	) (Worker, bool, error)

	FindWorkerForBuildContainer(
		logger lager.Logger,
		teamID int,
		buildID int,
		planID atc.PlanID,
	) (Worker, bool, error)
}

var (
	ErrNoWorkers     = errors.New("no workers")
	ErrMissingWorker = errors.New("worker for container is missing")
)

type NoCompatibleWorkersError struct {
	Spec    WorkerSpec
	Workers []Worker
}

func (err NoCompatibleWorkersError) Error() string {
	availableWorkers := ""
	for _, worker := range err.Workers {
		availableWorkers += "\n  - " + worker.Description()
	}

	return fmt.Sprintf(
		"no workers satisfying: %s\n\navailable workers: %s",
		err.Spec.Description(),
		availableWorkers,
	)
}

type pool struct {
	provider WorkerProvider

	rand *rand.Rand
}

func NewPool(provider WorkerProvider) Client {
	return &pool{
		provider: provider,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (pool *pool) RunningWorkers(logger lager.Logger) ([]Worker, error) {
	return pool.provider.RunningWorkers(logger)
}

func (pool *pool) AllSatisfying(logger lager.Logger, spec WorkerSpec, resourceTypes atc.VersionedResourceTypes) ([]Worker, error) {
	workers, err := pool.provider.RunningWorkers(logger)
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	compatibleTeamWorkers := []Worker{}
	compatibleGeneralWorkers := []Worker{}
	for _, worker := range workers {
		satisfyingWorker, err := worker.Satisfying(logger, spec, resourceTypes)
		if err == nil {
			if worker.IsOwnedByTeam() {
				compatibleTeamWorkers = append(compatibleTeamWorkers, satisfyingWorker)
			} else {
				compatibleGeneralWorkers = append(compatibleGeneralWorkers, satisfyingWorker)
			}
		}
	}

	if len(compatibleTeamWorkers) != 0 {
		return compatibleTeamWorkers, nil
	}

	if len(compatibleGeneralWorkers) != 0 {
		return compatibleGeneralWorkers, nil
	}

	return nil, NoCompatibleWorkersError{
		Spec:    spec,
		Workers: workers,
	}
}

func (pool *pool) Satisfying(logger lager.Logger, spec WorkerSpec, resourceTypes atc.VersionedResourceTypes) (Worker, error) {
	compatibleWorkers, err := pool.AllSatisfying(logger, spec, resourceTypes)
	if err != nil {
		return nil, err
	}
	randomWorker := compatibleWorkers[pool.rand.Intn(len(compatibleWorkers))]
	return randomWorker, nil
}

func (pool *pool) FindOrCreateContainer(
	logger lager.Logger,
	signals <-chan os.Signal,
	delegate ImageFetchingDelegate,
	user db.ResourceUser,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	spec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
) (Container, error) {
	worker, found, err := pool.provider.FindWorkerForContainerByOwner(
		logger.Session("find-worker"),
		spec.TeamID, // XXX: better place for this?
		owner,
	)
	if err != nil {
		return nil, err
	}

	if !found {
		compatibleWorkers, err := pool.AllSatisfying(logger, spec.WorkerSpec(), resourceTypes)
		if err != nil {
			return nil, err
		}

		workersByCount := map[int][]Worker{}
		var highestCount int
		for _, w := range compatibleWorkers {
			candidateInputCount := 0

			for _, inputSource := range spec.Inputs {
				_, found, err := inputSource.Source().VolumeOn(w)
				if err != nil {
					return nil, err
				}

				if found {
					candidateInputCount++
				}
			}

			workersByCount[candidateInputCount] = append(workersByCount[candidateInputCount], w)

			if candidateInputCount >= highestCount {
				highestCount = candidateInputCount
			}
		}

		workers := workersByCount[highestCount]

		worker = workers[pool.rand.Intn(len(workers))]
	}

	return worker.FindOrCreateContainer(
		logger,
		signals,
		delegate,
		user,
		owner,
		metadata,
		spec,
		resourceTypes,
	)
}

func (pool *pool) FindOrCreateBuildContainer(
	logger lager.Logger,
	signals <-chan os.Signal,
	delegate ImageFetchingDelegate,
	buildID int,
	planID atc.PlanID,
	metadata db.ContainerMetadata,
	spec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
) (Container, error) {
	worker, found, err := pool.provider.FindWorkerForBuildContainer(
		logger.Session("find-worker"),
		spec.TeamID, // XXX: better place for this?
		buildID,
		planID,
	)
	if err != nil {
		return nil, err
	}

	if !found {
		compatibleWorkers, err := pool.AllSatisfying(logger, spec.WorkerSpec(), resourceTypes)
		if err != nil {
			return nil, err
		}

		workersByCount := map[int][]Worker{}
		var highestCount int
		for _, w := range compatibleWorkers {
			candidateInputCount := 0

			for _, inputSource := range spec.Inputs {
				_, found, err := inputSource.Source().VolumeOn(w)
				if err != nil {
					return nil, err
				}

				if found {
					candidateInputCount++
				}
			}

			workersByCount[candidateInputCount] = append(workersByCount[candidateInputCount], w)

			if candidateInputCount >= highestCount {
				highestCount = candidateInputCount
			}
		}

		workers := workersByCount[highestCount]

		worker = workers[pool.rand.Intn(len(workers))]
	}

	return worker.FindOrCreateBuildContainer(
		logger,
		signals,
		delegate,
		buildID,
		planID,
		metadata,
		spec,
		resourceTypes,
	)
}

func (pool *pool) FindOrCreateResourceCheckContainer(
	logger lager.Logger,
	resourceUser db.ResourceUser,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	metadata db.ContainerMetadata,
	spec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
	resourceType string,
	source atc.Source,
) (Container, error) {
	worker, found, err := pool.provider.FindWorkerForResourceCheckContainer(
		logger.Session("find-worker"),
		spec.TeamID, // XXX: better place for this?
		resourceUser,
		resourceType,
		source,
		resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	if !found {
		worker, err = pool.Satisfying(logger, spec.WorkerSpec(), resourceTypes)
		if err != nil {
			return nil, err
		}
	}

	return worker.FindOrCreateResourceCheckContainer(
		logger,
		resourceUser,
		cancel,
		delegate,
		metadata,
		spec,
		resourceTypes,
		resourceType,
		source,
	)
}

func (pool *pool) FindContainerByHandle(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
	worker, found, err := pool.provider.FindWorkerForContainer(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.FindContainerByHandle(logger, teamID, handle)
}

func (*pool) FindResourceTypeByPath(string) (atc.WorkerResourceType, bool) {
	return atc.WorkerResourceType{}, false
}

func (*pool) CreateVolumeForResourceCache(lager.Logger, VolumeSpec, *db.UsedResourceCache) (Volume, error) {
	return nil, errors.New("CreateVolumeForResourceCache not implemented for pool")
}

func (*pool) FindInitializedVolumeForResourceCache(logger lager.Logger, resourceCache *db.UsedResourceCache) (Volume, bool, error) {
	return nil, false, errors.New("FindInitializedVolumeForResourceCache not implemented for pool")
}

func (*pool) LookupVolume(lager.Logger, string) (Volume, bool, error) {
	return nil, false, errors.New("LookupVolume not implemented for pool")
}

func resourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}
