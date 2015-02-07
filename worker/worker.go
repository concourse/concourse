package worker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/pivotal-golang/clock"
)

var ErrContainerNotFound = errors.New("container not found")

const containerKeepalive = 30 * time.Second

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int
}

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	Release()
}

type gardenWorker struct {
	gardenClient garden.Client
	clock        clock.Clock

	activeContainers int
}

func NewGardenWorker(gardenClient garden.Client, clock clock.Clock, activeContainers int) Worker {
	return &gardenWorker{
		gardenClient: gardenClient,
		clock:        clock,

		activeContainers: activeContainers,
	}
}

func (worker *gardenWorker) Create(spec garden.ContainerSpec) (Container, error) {
	gardenContainer, err := worker.gardenClient.Create(spec)
	if err != nil {
		return nil, err
	}

	return newGardenWorkerContainer(gardenContainer, worker.gardenClient, worker.clock), nil
}

func (worker *gardenWorker) Lookup(handle string) (Container, error) {
	gardenContainer, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		return nil, err
	}

	return newGardenWorkerContainer(gardenContainer, worker.gardenClient, worker.clock), nil
}

func (worker *gardenWorker) ActiveContainers() int {
	return worker.activeContainers
}

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client

	clock clock.Clock

	stopHeartbeating chan struct{}
	heartbeating     *sync.WaitGroup
}

func newGardenWorkerContainer(container garden.Container, gardenClient garden.Client, clock clock.Clock) Container {
	workerContainer := &gardenWorkerContainer{
		Container: container,

		gardenClient: gardenClient,

		clock: clock,

		heartbeating:     new(sync.WaitGroup),
		stopHeartbeating: make(chan struct{}),
	}

	workerContainer.heartbeating.Add(1)
	go workerContainer.heartbeat(clock.NewTicker(containerKeepalive))

	return workerContainer
}

func (container *gardenWorkerContainer) Destroy() error {
	container.Release()
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) Release() {
	close(container.stopHeartbeating)
	container.heartbeating.Wait()
}

func (container *gardenWorkerContainer) heartbeat(pacemaker clock.Ticker) {
	defer container.heartbeating.Done()

	for {
		select {
		case <-pacemaker.C():
			container.SetProperty("keepalive", fmt.Sprintf("%d", container.clock.Now().Unix()))
		case <-container.stopHeartbeating:
			return
		}
	}
}
