package worker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/pivotal-golang/clock"
)

var ErrContainerNotFound = errors.New("container not found")
var ErrUnsupportedResourceType = errors.New("unsupported resource type")

const containerKeepalive = 30 * time.Second

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int
	Satisfies(ContainerSpec) bool
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
	resourceTypes    []atc.WorkerResourceType
	platform         string
	tags             []string
}

func NewGardenWorker(
	gardenClient garden.Client,
	clock clock.Clock,
	activeContainers int,
	resourceTypes []atc.WorkerResourceType,
	platform string,
	tags []string,
) Worker {
	return &gardenWorker{
		gardenClient: gardenClient,
		clock:        clock,

		activeContainers: activeContainers,
		resourceTypes:    resourceTypes,
		platform:         platform,
		tags:             tags,
	}
}

func (worker *gardenWorker) CreateContainer(handle string, spec ContainerSpec) (Container, error) {
	gardenSpec := garden.ContainerSpec{
		Handle: handle,
	}

dance:
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		gardenSpec.Privileged = true

		for _, t := range worker.resourceTypes {
			if t.Type == s.Type {
				gardenSpec.RootFSPath = t.Image
				break dance
			}
		}

		return nil, ErrUnsupportedResourceType

	case ImageContainerSpec:
		gardenSpec.RootFSPath = s.Image
		gardenSpec.Privileged = s.Privileged

	default:
		return nil, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}

	gardenContainer, err := worker.gardenClient.Create(gardenSpec)
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

func (worker *gardenWorker) Satisfies(spec ContainerSpec) bool {
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		for _, t := range worker.resourceTypes {
			if t.Type == s.Type {
				return true
			}
		}

	case ImageContainerSpec:
		if s.Platform != worker.platform {
			return false
		}

	insert_coin:
		for _, stag := range s.Tags {
			for _, wtag := range worker.tags {
				if stag == wtag {
					continue insert_coin
				}
			}

			return false
		}

		return true
	}

	return false
}

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client

	clock clock.Clock

	stopHeartbeating chan struct{}
	heartbeating     *sync.WaitGroup

	releaseOnce sync.Once
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
	container.releaseOnce.Do(func() {
		close(container.stopHeartbeating)
		container.heartbeating.Wait()
	})
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
