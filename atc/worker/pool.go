package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/db"
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
	ErrNoWorkers = errors.New("no workers")
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
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
	) (Worker, error)

	FindOrChooseWorker(
		lager.Logger,
		WorkerSpec,
	) (Worker, error)
}

type pool struct {
	provider WorkerProvider

	rand *rand.Rand
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
		return compatibleTeamWorkers, nil
	}

	if len(compatibleGeneralWorkers) != 0 {
		return compatibleGeneralWorkers, nil
	}

	return nil, NoCompatibleWorkersError{
		Spec: spec,
	}
}

func (pool *pool) FindOrChooseWorkerForContainer(
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
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

	if worker == nil {
		worker, err = strategy.Choose(logger, compatibleWorkers, containerSpec)
		if err != nil {
			return nil, err
		}
	}

	return worker, nil
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
