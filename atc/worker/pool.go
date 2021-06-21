package worker

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/hashicorp/go-multierror"
)

const WorkerPollingInterval = 5 * time.Second

type NoCompatibleWorkersError struct {
	Spec WorkerSpec
}

func (err NoCompatibleWorkersError) Error() string {
	return fmt.Sprintf("no workers satisfying: %s", err.Spec.Description())
}

//counterfeiter:generate . Pool
type Pool interface {
	FindContainer(lager.Logger, int, string) (Container, bool, error)
	VolumeFinder
	CreateVolume(lager.Logger, VolumeSpec, WorkerSpec, db.VolumeType) (Volume, error)

	SelectWorker(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		PoolCallbacks,
	) (Client, time.Duration, error)

	FindWorkersForResourceCache(
		lager.Logger,
		int,
		int,
		WorkerSpec,
	) ([]Worker, error)

	ReleaseWorker(
		context.Context,
		ContainerSpec,
		Client,
		ContainerPlacementStrategy,
	)
}

//counterfeiter:generate . PoolCallbacks
type PoolCallbacks interface {
	WaitingForWorker(lager.Logger)
}

//counterfeiter:generate . VolumeFinder
type VolumeFinder interface {
	FindVolume(lager.Logger, int, string) (Volume, bool, error)
}

type pool struct {
	provider WorkerProvider
	waker    chan bool
}

func NewPool(provider WorkerProvider) Pool {
	return &pool{
		provider: provider,
		waker:    make(chan bool),
	}
}

func (pool *pool) allSatisfying(logger lager.Logger, spec WorkerSpec) ([]Worker, error) {
	workers, err := pool.provider.RunningWorkers(logger)
	if err != nil {
		return nil, err
	}

	return pool.compatibleWorkers(logger, workers, spec)
}

func (pool *pool) compatibleWorkers(logger lager.Logger, candidateWorkers []Worker, spec WorkerSpec) ([]Worker, error) {
	if len(candidateWorkers) == 0 {
		return candidateWorkers, nil
	}

	compatibleTeamWorkers := []Worker{}
	compatibleGeneralWorkers := []Worker{}
	for _, worker := range candidateWorkers {
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

	return compatibleGeneralWorkers, nil
}

func (pool *pool) findWorkerWithContainer(
	logger lager.Logger,
	compatible []Worker,
	owner db.ContainerOwner,
) (Worker, error) {
	workersWithContainer, err := pool.provider.FindWorkersForContainerByOwner(
		logger.Session("find-worker"),
		owner,
	)
	if err != nil {
		return nil, err
	}

	for _, worker := range compatible {
		for _, c := range workersWithContainer {
			if worker.Name() == c.Name() {
				logger.Debug("found-existing-container-on-worker", lager.Data{"worker": worker.Name()})
				return worker, nil
			}
		}
	}

	return nil, nil
}

func (pool *pool) findWorkerFromStrategy(
	logger lager.Logger,
	compatible []Worker,
	containerSpec ContainerSpec,
	strategy ContainerPlacementStrategy,
) (Worker, error) {
	orderedWorkers, err := strategy.Order(logger, compatible, containerSpec)

	if err != nil {
		return nil, err
	}

	var strategyError error
	for _, candidate := range orderedWorkers {
		err := strategy.Approve(logger, candidate, containerSpec)

		if err == nil {
			return candidate, nil
		}

		strategyError = multierror.Append(
			strategyError,
			fmt.Errorf("worker: %s, error: %v", candidate.Name(), err),
		)
	}

	logger.Debug("all-candidate-workers-rejected-during-selection", lager.Data{"reason": strategyError.Error()})
	return nil, nil
}

func (pool *pool) findWorker(
	ctx context.Context,
	containerOwner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
) (Client, error) {
	logger := lagerctx.FromContext(ctx)

	compatibleWorkers, err := pool.allSatisfying(logger, workerSpec)
	if err != nil {
		return nil, err
	}

	if len(compatibleWorkers) == 0 {
		return nil, nil
	}

	worker, err := pool.findWorkerWithContainer(
		logger,
		compatibleWorkers,
		containerOwner,
	)
	if err != nil {
		return nil, err
	}

	if worker == nil {
		worker, err = pool.findWorkerFromStrategy(
			logger,
			compatibleWorkers,
			containerSpec,
			strategy,
		)
		if err != nil {
			return nil, err
		}
	}

	if worker == nil {
		return nil, nil
	}

	return NewClient(worker), nil
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

func (pool *pool) FindWorkersForResourceCache(logger lager.Logger, teamId int, rcId int, workerSpec WorkerSpec) ([]Worker, error) {
	workers, err := pool.provider.FindWorkersForResourceCache(logger, teamId, rcId)
	if err != nil {
		return nil, err
	}

	return pool.compatibleWorkers(logger, workers, workerSpec)
}

func (pool *pool) SelectWorker(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	callbacks PoolCallbacks,
) (Client, time.Duration, error) {
	logger := lagerctx.FromContext(ctx)

	started := time.Now()
	labels := metric.StepsWaitingLabels{
		Platform:   workerSpec.Platform,
		TeamId:     strconv.Itoa(workerSpec.TeamID),
		TeamName:   containerSpec.TeamName,
		Type:       string(containerSpec.Type),
		WorkerTags: strings.Join(workerSpec.Tags, "_"),
	}

	var worker Client
	var pollingTicker *time.Ticker
	for {
		var err error
		worker, err = pool.findWorker(ctx, owner, containerSpec, workerSpec, strategy)

		if err != nil {
			return nil, 0, err
		}

		if worker != nil {
			break
		}

		if pollingTicker == nil {
			pollingTicker = time.NewTicker(WorkerPollingInterval)
			defer pollingTicker.Stop()

			logger.Debug("waiting-for-available-worker")

			_, ok := metric.Metrics.StepsWaiting[labels]
			if !ok {
				metric.Metrics.StepsWaiting[labels] = &metric.Gauge{}
			}

			metric.Metrics.StepsWaiting[labels].Inc()
			defer metric.Metrics.StepsWaiting[labels].Dec()

			if callbacks != nil {
				callbacks.WaitingForWorker(logger)
			}
		}

		select {
		case <-ctx.Done():
			logger.Info("aborted-waiting-for-worker")
			return nil, 0, ctx.Err()
		case <-pollingTicker.C:
		case <-pool.waker:
		}
	}

	elapsed := time.Since(started)
	metric.StepsWaitingDuration{
		Labels:   labels,
		Duration: elapsed,
	}.Emit(logger)

	return worker, elapsed, nil
}

func (pool *pool) ReleaseWorker(
	ctx context.Context,
	containerSpec ContainerSpec,
	client Client,
	strategy ContainerPlacementStrategy,
) {
	logger := lagerctx.FromContext(ctx)
	strategy.Release(logger, client.Worker(), containerSpec)

	// Attempt to wake a random waiting step to see if it can be
	// scheduled on the recently released worker.
	select {
	case pool.waker <- true:
		logger.Debug("attempted-to-wake-waiting-step")
	default:
	}
}

func (pool *pool) chooseRandomWorkerForVolume(
	logger lager.Logger,
	workerSpec WorkerSpec,
) (Worker, error) {
	workers, err := pool.allSatisfying(logger, workerSpec)
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, NoCompatibleWorkersError{Spec: workerSpec}
	}

	return workers[rand.Intn(len(workers))], nil
}
