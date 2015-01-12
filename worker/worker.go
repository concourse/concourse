package worker

import (
	"errors"

	garden "github.com/cloudfoundry-incubator/garden/api"
	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
)

var ErrContainerNotFound = errors.New("container not found")

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

	activeContainers int
}

func NewGardenWorker(addr string, activeContainers int) Worker {
	return &gardenWorker{
		gardenClient: gclient.New(gconn.New("tcp", addr)),

		activeContainers: activeContainers,
	}
}

func (worker *gardenWorker) Create(spec garden.ContainerSpec) (Container, error) {
	gardenContainer, err := worker.gardenClient.Create(spec)
	if err != nil {
		return nil, err
	}

	return &gardenWorkerContainer{
		Container: gardenContainer,

		gardenClient: worker.gardenClient,
	}, nil
}

func (worker *gardenWorker) Lookup(handle string) (Container, error) {
	gardenContainer, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		return nil, err
	}

	return &gardenWorkerContainer{
		Container: gardenContainer,

		gardenClient: worker.gardenClient,
	}, nil
}

func (worker *gardenWorker) ActiveContainers() int {
	return worker.activeContainers
}

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client
}

func (container *gardenWorkerContainer) Destroy() error {
	return container.gardenClient.Destroy(container.Handle())
}
