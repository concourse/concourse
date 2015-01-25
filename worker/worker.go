package worker

import (
	"errors"
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/vito/clock"
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
}

func (container *gardenWorkerContainer) Destroy() error {
	close(container.stopHeartbeating)
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) heartbeat(pacemaker clock.Ticker) {
	for {
		select {
		case <-pacemaker.C():
			container.SetProperty("keepalive", fmt.Sprintf("%d", container.clock.Now().Unix()))
		case <-container.stopHeartbeating:
			return
		}
	}
}

func newGardenWorkerContainer(container garden.Container, gardenClient garden.Client, clock clock.Clock) Container {
	workerContainer := &gardenWorkerContainer{
		Container: container,

		gardenClient: gardenClient,

		clock:            clock,
		stopHeartbeating: make(chan struct{}),
	}

	go workerContainer.heartbeat(clock.NewTicker(containerKeepalive))

	return workerContainer
}
