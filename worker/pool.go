package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
)

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	Workers() ([]Worker, error)
}

var ErrNoWorkers = errors.New("no workers")

type NoCompatibleWorkersError struct {
	Spec    ContainerSpec
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

type Pool struct {
	provider WorkerProvider

	rand *rand.Rand
}

func NewPool(provider WorkerProvider) Client {
	return &Pool{
		provider: provider,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (pool *Pool) CreateContainer(id Identifier, spec ContainerSpec) (Container, error) {
	workers, err := pool.provider.Workers()
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	compatibleWorkers := []Worker{}
	for _, worker := range workers {
		if worker.Satisfies(spec) {
			compatibleWorkers = append(compatibleWorkers, worker)
		}
	}

	if len(compatibleWorkers) == 0 {
		return nil, NoCompatibleWorkersError{
			Spec:    spec,
			Workers: workers,
		}
	}

	randomWorker := compatibleWorkers[pool.rand.Intn(len(compatibleWorkers))]

	return randomWorker.CreateContainer(id, spec)
}

func (pool *Pool) FindContainerForIdentifier(id Identifier) (Container, error) {
	workers, err := pool.provider.Workers()
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(workers))

	found := make(chan Container, len(workers))
	multiErrs := make(chan MultipleContainersError, len(workers))

	for _, worker := range workers {
		go func(worker Worker) {
			defer wg.Done()

			container, err := worker.FindContainerForIdentifier(id)
			if err == nil {
				found <- container
			} else if multi, ok := err.(MultipleContainersError); ok {
				multiErrs <- multi
			}
		}(worker)
	}

	wg.Wait()

	totalFound := len(found)
	totalMulti := len(multiErrs)

	if totalMulti != 0 {
		allHandles := []string{}

		for i := 0; i < totalMulti; i++ {
			multiErr := <-multiErrs

			allHandles = append(allHandles, multiErr.Handles...)
		}

		for i := 0; i < totalFound; i++ {
			c := <-found
			allHandles = append(allHandles, c.Handle())
			c.Release()
		}

		return nil, MultipleContainersError{allHandles}
	}

	switch totalFound {
	case 0:
		return nil, ErrContainerNotFound
	case 1:
		return <-found, nil
	default:
		handles := make([]string, totalFound)

		for i := 0; i < totalFound; i++ {
			c := <-found

			handles[i] = c.Handle()

			c.Release()
		}

		return nil, MultipleContainersError{Handles: handles}
	}
}

type workerErrorInfo struct {
	workerName string
	err        error
}

func (pool *Pool) FindContainersForIdentifier(id Identifier) ([]Container, error) {
	workers, err := pool.provider.Workers()
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(workers))

	found := make(chan []Container, len(workers))
	errors := make(chan workerErrorInfo, len(workers))

	for _, worker := range workers {
		go func(worker Worker) {
			defer wg.Done()

			containers, err := worker.FindContainersForIdentifier(id)
			found <- containers
			if err != nil {
				errors <- workerErrorInfo{
					workerName: worker.Name(),
					err:        err,
				}
			}
		}(worker)
	}

	wg.Wait()

	totalFound := len(found)

	containers := []Container{}
	for i := 0; i < totalFound; i++ {
		foundContainers := <-found
		for _, c := range foundContainers {
			containers = append(containers, c)
		}
	}

	totalErr := len(errors)
	if len(errors) > 0 {
		multiWorkerError := MultiWorkerError{}

		for i := 0; i < totalErr; i++ {
			e := <-errors
			multiWorkerError.AddError(e.workerName, e.err)
		}
		return containers, multiWorkerError
	}

	return containers, nil
}

type foundContainer struct {
	workerName string
	container  Container
}

func (pool *Pool) LookupContainer(handle string) (Container, error) {
	workers, err := pool.provider.Workers()
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(workers))

	found := make(chan foundContainer, len(workers))
	errors := make(chan workerErrorInfo, len(workers))

	for _, worker := range workers {
		go func(worker Worker) {
			defer wg.Done()

			container, err := worker.LookupContainer(handle)
			if container != nil {
				found <- foundContainer{
					workerName: worker.Name(),
					container:  container,
				}
			}
			if err != nil {
				_, ok := err.(garden.ContainerNotFoundError)
				if !ok {
					errors <- workerErrorInfo{
						workerName: worker.Name(),
						err:        err,
					}
				}
			}
		}(worker)
	}

	wg.Wait()

	totalErrors := len(errors)

	var multiWorkerError *MultiWorkerError
	if totalErrors != 0 {
		allErrors := make(map[string]error, totalErrors)

		for i := 0; i < totalErrors; i++ {
			e := <-errors

			allErrors[e.workerName] = e.err
		}

		multiWorkerError = &MultiWorkerError{allErrors}
	}

	totalFound := len(found)
	switch totalFound {
	case 0:
		return nil, garden.ContainerNotFoundError{}
	case 1:
		return (<-found).container, multiWorkerError
	default:
		names := make([]string, totalFound)

		for i := 0; i < totalFound; i++ {
			c := <-found
			names[i] = c.workerName
			if c.container != nil {
				c.container.Release()
			}
		}

		return nil, MultipleWorkersFoundContainerError{Names: names}
	}
}

func (pool *Pool) Name() string {
	return ""
}

type byActiveContainers []Worker

func (cs byActiveContainers) Len() int { return len(cs) }

func (cs byActiveContainers) Swap(i, j int) { cs[i], cs[j] = cs[j], cs[i] }

func (cs byActiveContainers) Less(i, j int) bool {
	return cs[i].ActiveContainers() < cs[j].ActiveContainers()
}
