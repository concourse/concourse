package worker

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

//go:generate counterfeiter . Client

type Client interface {
	CreateContainer(handle string, spec ContainerSpec) (Container, error)
	Lookup(handle string) (Container, error)
}

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	Workers() ([]Worker, error)
}

var ErrNoWorkers = errors.New("no workers")

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

func (pool *Pool) CreateContainer(handle string, spec ContainerSpec) (Container, error) {
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

	return compatibleWorkers[pool.rand.Intn(len(compatibleWorkers))].CreateContainer(handle, spec)
}

func (pool *Pool) Lookup(handle string) (Container, error) {
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

	for _, worker := range workers {
		go func(worker Worker) {
			defer wg.Done()

			container, err := worker.Lookup(handle)
			if err == nil {
				found <- container
			}
		}(worker)
	}

	wg.Wait()

	select {
	case container := <-found:
		return container, nil
	default:
		return nil, ErrContainerNotFound
	}
}

type byActiveContainers []Worker

func (cs byActiveContainers) Len() int { return len(cs) }

func (cs byActiveContainers) Swap(i, j int) { cs[i], cs[j] = cs[j], cs[i] }

func (cs byActiveContainers) Less(i, j int) bool {
	return cs[i].ActiveContainers() < cs[j].ActiveContainers()
}
