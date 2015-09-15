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
}

var (
	ErrNoWorkers        = errors.New("no workers")
	ErrDBGardenMismatch = errors.New("discrepency between db and garden worker containers found")
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
	logger   lager.Logger

	rand *rand.Rand
}

func NewPool(provider WorkerProvider, logger lager.Logger) Client {
	return &pool{
		provider: provider,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:   logger,
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

func (pool *pool) CreateContainer(id Identifier, spec ContainerSpec) (Container, error) {
	worker, err := pool.Satisfying(spec.WorkerSpec())
	if err != nil {
		return nil, err
	}

	container, err := worker.CreateContainer(id, spec)
	if err != nil {
		return nil, err
	}

	return container, nil
}

func (pool *pool) FindContainerForIdentifier(id Identifier) (Container, bool, error) {
	pool.logger.Info("finding container for identifier", lager.Data{"identifier": id})

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
		err = ErrDBGardenMismatch
		pool.logger.Error("found container belonging to worker that does not exist in the db",
			err, lager.Data{"workerName": containerInfo.WorkerName})
		return nil, false, err
	}

	container, found, err := worker.LookupContainer(containerInfo.Handle)
	if err != nil {
		return nil, false, err
	}
	if !found {
		err = ErrDBGardenMismatch
		pool.logger.Error("found container in db that does not exist in garden",
			err, lager.Data{"containerName": containerInfo.Name})
		return nil, false, err
	}

	return container, found, nil
}

type workerErrorInfo struct {
	workerName string
	err        error
}

type foundContainer struct {
	workerName string
	container  Container
}

func (pool *pool) LookupContainer(handle string) (Container, bool, error) {
	pool.logger.Info("looking up container", lager.Data{"handle": handle})

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
		err = ErrDBGardenMismatch
		pool.logger.Error("found container belonging to worker that does not exist in the db",
			err, lager.Data{"workerName": containerInfo.WorkerName})
		return nil, false, err
	}

	container, found, err := worker.LookupContainer(handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		err = ErrDBGardenMismatch
		pool.logger.Error("found container in db that does not exist in garden",
			err, lager.Data{"containerName": containerInfo.Name})
		return nil, false, err
	}

	return container, found, nil
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
