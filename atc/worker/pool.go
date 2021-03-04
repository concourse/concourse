package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
)

const workerPollingInterval = 5 * time.Second

var (
	ErrNoWorkers           = errors.New("no workers")
	ErrFailedToAcquireLock = errors.New("failed to acquire lock")
	ErrFailedToPickWorker  = errors.New("failed to pick worker")
)

type NoCompatibleWorkersError struct {
	Spec WorkerSpec
}

func (err NoCompatibleWorkersError) Error() string {
	return fmt.Sprintf("no workers satisfying: %s", err.Spec.Description())
}

//go:generate counterfeiter . Pool

type Pool interface {
	FindContainer(lager.Logger, int, string) (Container, bool, error)
	VolumeFinder
	CreateVolume(lager.Logger, VolumeSpec, WorkerSpec, db.VolumeType) (Volume, error)

	ContainerInWorker(lager.Logger, db.ContainerOwner, WorkerSpec) (bool, error)

	SelectWorker(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		PoolCallbacks,
	) (Client, error)

	WaitForWorker(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		PoolCallbacks,
	) (Client, time.Duration, error)
}

//go:generate counterfeiter . PoolCallbacks

type PoolCallbacks interface {
	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, Worker)
}

//go:generate counterfeiter . VolumeFinder

type VolumeFinder interface {
	FindVolume(lager.Logger, int, string) (Volume, bool, error)
}

type pool struct {
	provider    WorkerProvider
	lockFactory lock.LockFactory

	rand *rand.Rand
}

func NewPool(provider WorkerProvider, lockFactory lock.LockFactory) Pool {
	return &pool{
		provider:    provider,
		lockFactory: lockFactory,

		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
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
	for _, worker := range workers {
		compatible := worker.Satisfies(logger, spec)
		if compatible {
			if worker.IsOwnedByTeam() {
				compatibleTeamWorkers = append(compatibleTeamWorkers, worker)
			} else {
				compatibleGeneralWorkers = append(compatibleGeneralWorkers, worker)
			}
		}
	}

	if len(compatibleTeamWorkers) != 0 {
		// XXX(aoldershaw): if there is a team worker that is compatible but is
		// rejected by the strategy, shouldn't we fallback to general workers?
		return compatibleTeamWorkers, nil
	}

	if len(compatibleGeneralWorkers) != 0 {
		return compatibleGeneralWorkers, nil
	}

	return nil, NoCompatibleWorkersError{
		Spec: spec,
	}
}

func (pool *pool) FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
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

func (pool *pool) FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error) {
	worker, found, err := pool.provider.FindWorkerForVolume(
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

	return worker.LookupVolume(logger, handle)
}

func (pool *pool) CreateVolume(logger lager.Logger, volumeSpec VolumeSpec, workerSpec WorkerSpec, volumeType db.VolumeType) (Volume, error) {
	worker, err := pool.chooseRandomWorkerForVolume(logger, workerSpec)
	if err != nil {
		return nil, err
	}

	return worker.CreateVolume(logger, volumeSpec, workerSpec.TeamID, volumeType)
}

func (pool *pool) ContainerInWorker(logger lager.Logger, owner db.ContainerOwner, workerSpec WorkerSpec) (bool, error) {
	workersWithContainer, err := pool.provider.FindWorkersForContainerByOwner(
		logger.Session("find-worker"),
		owner,
	)
	if err != nil {
		return false, err
	}

	compatibleWorkers, err := pool.allSatisfying(logger, workerSpec)
	if err != nil {
		return false, err
	}

	for _, w := range workersWithContainer {
		for _, c := range compatibleWorkers {
			if w.Name() == c.Name() {
				return true, nil
			}
		}
	}

	return false, nil
}

func (pool *pool) SelectWorker(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	callbacks PoolCallbacks,
) (Client, error) {
	logger := lagerctx.FromContext(ctx)

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

	// Lock required to protect call to strategy.Pick and callbacks.SelectedWorker
	//
	// strategy.Pick may rely on worker metrics (such as active task, container, and
	// volume counts) that may be modified by callbacks.SelectedWorker
	lock, lockAcquired, err := pool.lockFactory.Acquire(logger, lock.NewPlacementStrategyLockID())
	if err != nil {
		return nil, err
	}

	if !lockAcquired {
		return nil, ErrFailedToAcquireLock
	}

	defer lock.Release()

	if worker == nil {
		candidates, err := strategy.Candidates(logger, compatibleWorkers, containerSpec)

		if err != nil {
			return nil, err
		}

		for _, candidate := range candidates {
			err := strategy.Pick(logger, candidate, containerSpec)

			if err != nil {
				logger.Error("Candidate worker rejected due to error", err)
			} else {
				worker = candidate
				break
			}
		}

		if worker == nil {
			return nil, ErrFailedToPickWorker
		}
	}

	callbacks.SelectedWorker(logger, worker)
	return NewClient(worker), nil
}

func (pool *pool) WaitForWorker(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	callbacks PoolCallbacks,
) (Client, time.Duration, error) {
	logger := lagerctx.FromContext(ctx)

	started := time.Now()
	pollingTicker := time.NewTicker(workerPollingInterval)
	defer pollingTicker.Stop()

	labels := metric.StepsWaitingLabels{
		TeamId:     strconv.Itoa(workerSpec.TeamID),
		WorkerTags: strings.Join(workerSpec.Tags, "_"),
		Platform:   workerSpec.Platform,
	}

	var worker Client
	var waiting bool = false
	for {
		var err error
		worker, err = pool.SelectWorker(ctx, owner, containerSpec, workerSpec, strategy, callbacks)

		if err != nil {
			if errors.Is(err, ErrNoWorkers) {
				// Could use these blocks to notify caller of the reason we're waiting
			} else if errors.Is(err, ErrFailedToAcquireLock) {
				//
			} else if errors.Is(err, ErrFailedToPickWorker) {
				//
			} else if errors.As(err, &NoCompatibleWorkersError{}) {
				//
			} else if errors.As(err, &NoWorkerFitContainerPlacementStrategyError{}) {
				//
			} else {
				return nil, 0, err
			}
		}

		if worker != nil {
			break
		}

		if !waiting {
			_, ok := metric.Metrics.StepsWaiting[labels]
			if !ok {
				metric.Metrics.StepsWaiting[labels] = &metric.Gauge{}
			}

			metric.Metrics.StepsWaiting[labels].Inc()
			defer metric.Metrics.StepsWaiting[labels].Dec()

			if callbacks != nil {
				callbacks.WaitingForWorker(logger)
			}

			waiting = true
		}

		select {
		case <-ctx.Done():
			logger.Info("aborted-waiting-worker")
			return nil, 0, ctx.Err()
		case <-pollingTicker.C:
			break
		}
	}

	elapsed := time.Since(started)
	metric.StepsWaitingDuration{
		Labels:   labels,
		Duration: elapsed,
	}.Emit(logger)

	return worker, elapsed, nil
}

func (pool *pool) chooseRandomWorkerForVolume(
	logger lager.Logger,
	workerSpec WorkerSpec,
) (Worker, error) {
	workers, err := pool.allSatisfying(logger, workerSpec)
	if err != nil {
		return nil, err
	}

	return workers[rand.Intn(len(workers))], nil
}
