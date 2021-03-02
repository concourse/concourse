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
	"github.com/concourse/concourse/atc/metric"
)

const workerPollingInterval = 5 * time.Second

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
	) (Client, error)

	WaitForWorker(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
	) (Client, time.Duration, error)
}

//go:generate counterfeiter . VolumeFinder

type VolumeFinder interface {
	FindVolume(lager.Logger, int, string) (Volume, bool, error)
}

type pool struct {
	provider WorkerProvider
	rand     *rand.Rand
}

func NewPool(provider WorkerProvider) Pool {
	return &pool{
		provider: provider,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
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

	if worker == nil {
		worker, err = strategy.Choose(logger, compatibleWorkers, containerSpec)
		if err != nil {
			return nil, err
		}
	}
	return NewClient(worker), nil
}

func (pool *pool) WaitForWorker(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
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
		worker, err := pool.SelectWorker(ctx, owner, containerSpec, workerSpec, strategy)

		if err != nil {
			return nil, 0, err
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
