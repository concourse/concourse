package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	Workers() ([]Worker, error)
	GetWorker(string) (Worker, bool, error)
	FindContainerInfoForIdentifier(Identifier) (db.ContainerInfo, bool, error)
	GetContainerInfo(string) (db.ContainerInfo, bool, error)
	ReapContainer(string) error
}

var (
	ErrNoWorkers        = errors.New("no workers")
	ErrDBGardenMismatch = errors.New("discrepancy between db and garden worker containers found")
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

func (pool *pool) Satisfying(spec WorkerSpec) (Worker, error) {
	workers, err := pool.provider.Workers()
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	compatibleWorkers := []Worker{}
	for _, worker := range workers {
		satisfyingWorker, err := worker.Satisfying(spec)
		if err == nil {
			compatibleWorkers = append(compatibleWorkers, satisfyingWorker)
		}
	}

	if len(compatibleWorkers) == 0 {
		return nil, NoCompatibleWorkersError{
			Spec:    spec,
			Workers: workers,
		}
	}

	randomWorker := compatibleWorkers[pool.rand.Intn(len(compatibleWorkers))]

	return randomWorker, nil
}

func (pool *pool) CreateContainer(logger lager.Logger, id Identifier, spec ContainerSpec) (Container, error) {
	worker, err := pool.Satisfying(spec.WorkerSpec())
	if err != nil {
		return nil, err
	}

	container, err := worker.CreateContainer(logger, id, spec)
	if err != nil {
		return nil, err
	}

	return container, nil
}

func (pool *pool) FindContainerForIdentifier(logger lager.Logger, id Identifier) (Container, bool, error) {
	containerInfo, found, err := pool.provider.FindContainerInfoForIdentifier(id)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, found, nil
	}

	worker, found, err := pool.provider.GetWorker(containerInfo.WorkerName)
	if err != nil {
		return nil, found, err
	}

	if !found {
		logger.Info("found-container-for-missing-worker", lager.Data{
			"container-handle": containerInfo.Handle,
			"worker-name":      containerInfo.WorkerName,
		})

		return nil, false, ErrDBGardenMismatch
	}

	container, found, err := worker.LookupContainer(logger, containerInfo.Handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		logger.Info("found-container-in-db-but-not-on-worker", lager.Data{
			"container-handle": containerInfo.Handle,
			"worker-name":      containerInfo.WorkerName,
		})

		err := pool.provider.ReapContainer(containerInfo.Handle)
		if err != nil {
			return nil, false, err
		}

		return nil, false, err
	}

	return container, true, nil
}

func (pool *pool) LookupContainer(logger lager.Logger, handle string) (Container, bool, error) {
	logger.Info("looking-up-container", lager.Data{"handle": handle})

	containerInfo, found, err := pool.provider.GetContainerInfo(handle)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	worker, found, err := pool.provider.GetWorker(containerInfo.WorkerName)
	if err != nil {
		return nil, false, err
	}

	if !found {
		logger.Info("found-container-for-missing-worker", lager.Data{
			"container-handle": containerInfo.Handle,
			"worker-name":      containerInfo.WorkerName,
		})

		return nil, false, ErrDBGardenMismatch
	}

	container, found, err := worker.LookupContainer(logger, handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		logger.Info("found-container-in-db-but-not-on-worker", lager.Data{
			"container-handle": containerInfo.Handle,
			"worker-name":      containerInfo.WorkerName,
		})

		err := pool.provider.ReapContainer(handle)
		if err != nil {
			return nil, false, err
		}

		return nil, false, nil
	}

	return container, true, nil
}

func (pool *pool) Name() string {
	return "pool"
}

type byActiveContainers []Worker

func (cs byActiveContainers) Len() int { return len(cs) }

func (cs byActiveContainers) Swap(i, j int) { cs[i], cs[j] = cs[j], cs[i] }

func (cs byActiveContainers) Less(i, j int) bool {
	return cs[i].ActiveContainers() < cs[j].ActiveContainers()
}
