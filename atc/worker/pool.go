package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	RunningWorkers(lager.Logger) ([]Worker, error)

	FindWorkerForContainer(
		logger lager.Logger,
		teamID int,
		handle string,
	) (Worker, bool, error)

	FindWorkerForVolume(
		logger lager.Logger,
		teamID int,
		handle string,
	) (Worker, bool, error)

	FindWorkersForContainerByOwner(
		logger lager.Logger,
		owner db.ContainerOwner,
	) ([]Worker, error)

	NewGardenWorker(
		logger lager.Logger,
		tikTok clock.Clock,
		savedWorker db.Worker,
		numBuildWorkers int,
	) Worker
}

var (
	ErrNoWorkers             = errors.New("no workers")
	ErrFailedAcquirePoolLock = errors.New("failed to acquire pool lock")
)

type NoCompatibleWorkersError struct {
	Spec WorkerSpec
}

func (err NoCompatibleWorkersError) Error() string {
	return fmt.Sprintf("no workers satisfying: %s", err.Spec.Description())
}

//go:generate counterfeiter . Pool

type Pool interface {
	FindOrChooseWorkerForContainer(
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		db.ContainerMetadata,
		WorkerSpec,
		ContainerPlacementStrategy,
	) (Worker, error)

	FindOrChooseWorker(
		lager.Logger,
		WorkerSpec,
	) (Worker, error)

	AcquireContainerCreatingLock(
		logger lager.Logger,
	) (lock.Lock, bool, error)
}

type pool struct {
	clock       clock.Clock
	lockFactory lock.LockFactory
	provider    WorkerProvider

	rand *rand.Rand
}

func NewPool(
	clock clock.Clock,
	lockFactory lock.LockFactory,
	provider WorkerProvider,
) Pool {
	return &pool{
		clock:       clock,
		lockFactory: lockFactory,
		provider:    provider,
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (pool *pool) allSatisfying(logger lager.Logger, spec WorkerSpec) ([]Worker, error) {
	workers, err := pool.provider.RunningWorkers(logger)
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	compatibleTeamWorkers := []Worker{}
	compatibleGeneralWorkers := []Worker{}
	busyWorkers := 0
	compatibleWorkers := 0
	for _, worker := range workers {
		compatible := worker.Satisfies(logger, spec)
		if compatible {
			compatibleWorkers += 1
			if worker.BuildContainers() >= 1 { // TODO: from configuration
				busyWorkers += 1
				continue
			}
			if worker.IsOwnedByTeam() {
				compatibleTeamWorkers = append(compatibleTeamWorkers, worker)
			} else {
				compatibleGeneralWorkers = append(compatibleGeneralWorkers, worker)
			}
		}
	}

	if len(compatibleTeamWorkers) != 0 {
		return compatibleTeamWorkers, nil
	}

	if len(compatibleGeneralWorkers) != 0 {
		return compatibleGeneralWorkers, nil
	}

	if busyWorkers == compatibleWorkers {
		logger.Info(fmt.Sprintf("All the %d workers satisfying the task criteria are busy, please stand-by.", compatibleWorkers))
		pool.clock.Sleep(10 * time.Second)
		return pool.allSatisfying(logger, spec)
	}

	return nil, NoCompatibleWorkersError{
		Spec: spec,
	}
}

func (pool *pool) FindOrChooseWorkerForContainer(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	metadata db.ContainerMetadata,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
) (Worker, error) {
	workersWithContainer, err := pool.provider.FindWorkersForContainerByOwner(
		logger.Session("find-worker"),
		owner,
	)
	if err != nil {
		return nil, err
	}

	compatibleWorkers, err := pool.allSatisfying(logger, workerSpec)
	if err != nil {
		return nil, err
	}

	var worker Worker
dance:
	for _, w := range workersWithContainer {
		for _, c := range compatibleWorkers {
			if w.Name() == c.Name() {
				worker = c
				break dance
			}
		}
	}

	// pool is shared by all steps running in the system,
	// lock around worker placement strategies so decisions
	// are serialized and valid at the time of creating
	// containers in garden
	for {
		lock, acquired, err := pool.AcquireContainerCreatingLock(logger)
		if err != nil {
			return nil, ErrFailedAcquirePoolLock
		}

		if !acquired {
			pool.clock.Sleep(time.Second)
			continue
		}
		defer lock.Release()

		if worker == nil {
			worker, err = strategy.Choose(logger, compatibleWorkers, containerSpec)
			if err != nil {
				return nil, err
			}
		}

		err = worker.EnsureDBContainerExists(nil, logger, owner, metadata)
		if err != nil {
			return nil, err
		}
		break
	}

	return worker, nil
}

func (pool *pool) AcquireContainerCreatingLock(logger lager.Logger) (lock.Lock, bool, error) {
	return pool.lockFactory.Acquire(logger, lock.NewContainerCreatingLockID())
}

func (pool *pool) FindOrChooseWorker(
	logger lager.Logger,
	workerSpec WorkerSpec,
) (Worker, error) {
	workers, err := pool.allSatisfying(logger, workerSpec)
	if err != nil {
		return nil, err
	}

	return workers[rand.Intn(len(workers))], nil
}
