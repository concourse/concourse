package worker

import (
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/clock"
)

var ErrContainerNotFound = errors.New("container not found")
var ErrUnsupportedResourceType = errors.New("unsupported resource type")
var ErrIncompatiblePlatform = errors.New("incompatible platform")
var ErrMismatchedTags = errors.New("mismatched tags")

const containerKeepalive = 30 * time.Second

const ephemeralPropertyName = "concourse:ephemeral"

var trackedContainers = expvar.NewInt("TrackedContainers")

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int

	Description() string

	VolumeManager() (baggageclaim.Client, bool)
}

type gardenWorker struct {
	gardenClient       garden.Client
	baggageclaimClient baggageclaim.Client

	clock clock.Clock

	activeContainers int
	resourceTypes    []atc.WorkerResourceType
	platform         string
	tags             []string
	name             string
}

func NewGardenWorker(
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	clock clock.Clock,
	activeContainers int,
	resourceTypes []atc.WorkerResourceType,
	platform string,
	tags []string,
	name string,
) Worker {
	return &gardenWorker{
		gardenClient:       gardenClient,
		baggageclaimClient: baggageclaimClient,
		clock:              clock,

		activeContainers: activeContainers,
		resourceTypes:    resourceTypes,
		platform:         platform,
		tags:             tags,
		name:             name,
	}
}

func (worker *gardenWorker) VolumeManager() (baggageclaim.Client, bool) {
	if worker.baggageclaimClient != nil {
		return worker.baggageclaimClient, true
	} else {
		return nil, false
	}
}

func (worker *gardenWorker) CreateContainer(id Identifier, spec ContainerSpec) (Container, error) {
	gardenSpec := garden.ContainerSpec{
		Properties: id.gardenProperties(),
	}

dance:
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		gardenSpec.Privileged = true

		gardenSpec.Env = s.Env

		if s.Ephemeral {
			gardenSpec.Properties[ephemeralPropertyName] = "true"
		}

		if s.Volume != nil && s.MountPath != "" {
			gardenSpec.BindMounts = []garden.BindMount{
				{
					SrcPath: s.Volume.Path(),
					DstPath: s.MountPath,
					Mode:    garden.BindMountModeRW,
				},
			}
		}

		for _, t := range worker.resourceTypes {
			if t.Type == s.Type {
				gardenSpec.RootFSPath = t.Image
				break dance
			}
		}

		return nil, ErrUnsupportedResourceType

	case TaskContainerSpec:
		gardenSpec.RootFSPath = s.Image
		gardenSpec.Privileged = s.Privileged

	default:
		return nil, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}

	gardenContainer, err := worker.gardenClient.Create(gardenSpec)
	if err != nil {
		return nil, err
	}

	return newGardenWorkerContainer(gardenContainer, worker.gardenClient, worker.clock)
}

func (worker *gardenWorker) FindContainerForIdentifier(id Identifier) (Container, error) {
	containers, err := worker.gardenClient.Containers(id.gardenProperties())
	if err != nil {
		return nil, err
	}

	switch len(containers) {
	case 0:
		return nil, ErrContainerNotFound
	case 1:
		return newGardenWorkerContainer(containers[0], worker.gardenClient, worker.clock)
	default:
		handles := []string{}

		for _, c := range containers {
			handles = append(handles, c.Handle())
		}

		return nil, MultipleContainersError{
			Handles: handles,
		}
	}
}

func (worker *gardenWorker) FindContainersForIdentifier(id Identifier) ([]Container, error) {
	containers, err := worker.gardenClient.Containers(id.gardenProperties())
	if err != nil {
		return nil, err
	}

	gardenContainers := make([]Container, len(containers))
	for i, c := range containers {
		gardenContainers[i], err = newGardenWorkerContainer(c, worker.gardenClient, worker.clock)
		if err != nil {
			return nil, err
		}
	}

	return gardenContainers, nil
}

func (worker *gardenWorker) LookupContainer(handle string) (Container, error) {
	container, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		return nil, err
	}
	return newGardenWorkerContainer(container, worker.gardenClient, worker.clock)
}

func (worker *gardenWorker) ActiveContainers() int {
	return worker.activeContainers
}

func (worker *gardenWorker) Satisfying(spec WorkerSpec) (Worker, error) {
	if spec.ResourceType != "" {
		matchedType := false
		for _, t := range worker.resourceTypes {
			if t.Type == spec.ResourceType {
				matchedType = true
				break
			}
		}

		if !matchedType {
			return nil, ErrUnsupportedResourceType
		}
	}

	if spec.Platform != "" {
		if spec.Platform != worker.platform {
			return nil, ErrIncompatiblePlatform
		}
	}

	if !worker.tagsMatch(spec.Tags) {
		return nil, ErrMismatchedTags
	}

	return worker, nil
}

func (worker *gardenWorker) tagsMatch(tags []string) bool {
	if len(worker.tags) > 0 && len(tags) == 0 {
		return false
	}

insert_coin:
	for _, stag := range tags {
		for _, wtag := range worker.tags {
			if stag == wtag {
				continue insert_coin
			}
		}

		return false
	}

	return true
}

func (worker *gardenWorker) Description() string {
	messages := []string{
		fmt.Sprintf("platform '%s'", worker.platform),
	}

	for _, tag := range worker.tags {
		messages = append(messages, fmt.Sprintf("tag '%s'", tag))
	}

	return strings.Join(messages, ", ")
}

func (worker *gardenWorker) Name() string {
	return worker.name
}

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client

	clock clock.Clock

	stopHeartbeating chan struct{}
	heartbeating     *sync.WaitGroup

	releaseOnce sync.Once

	identifier Identifier
}

func newGardenWorkerContainer(container garden.Container, gardenClient garden.Client, clock clock.Clock) (Container, error) {
	workerContainer := &gardenWorkerContainer{
		Container: container,

		gardenClient: gardenClient,

		clock: clock,

		heartbeating:     new(sync.WaitGroup),
		stopHeartbeating: make(chan struct{}),
	}

	workerContainer.heartbeating.Add(1)
	go workerContainer.heartbeat(clock.NewTicker(containerKeepalive))

	trackedContainers.Add(1)
	metric.TrackedContainers.Inc()

	err := workerContainer.initializeIdentifier()
	if err != nil {
		workerContainer.Release()
		return nil, err
	}
	return workerContainer, nil
}

func (container *gardenWorkerContainer) Destroy() error {
	container.Release()
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) Release() {
	container.releaseOnce.Do(func() {
		close(container.stopHeartbeating)
		container.heartbeating.Wait()
		trackedContainers.Add(-1)
		metric.TrackedContainers.Dec()
	})
}

func (container *gardenWorkerContainer) initializeIdentifier() error {
	properties, err := container.Properties()
	if err != nil {
		fmt.Println("ERROR HERE")
		return err
	}

	propertyPrefix := "concourse:"
	identifier := Identifier{}

	nameKey := propertyPrefix + "name"
	if properties[nameKey] != "" {
		identifier.Name = properties[nameKey]
	}

	pipelineKey := propertyPrefix + "pipeline-name"
	if properties[pipelineKey] != "" {
		identifier.PipelineName = properties[pipelineKey]
	}

	buildIDKey := propertyPrefix + "build-id"
	if properties[buildIDKey] != "" {
		identifier.BuildID, err = strconv.Atoi(properties[buildIDKey])
		if err != nil {
			return err
		}
	}

	typeKey := propertyPrefix + "type"
	if properties[typeKey] != "" {
		identifier.Type = ContainerType(properties[typeKey])
	}

	stepLocationKey := propertyPrefix + "location"
	if properties[stepLocationKey] != "" {
		StepLocationUint, err := strconv.Atoi(properties[stepLocationKey])
		if err != nil {
			return err
		}
		identifier.StepLocation = uint(StepLocationUint)
	}

	checkTypeKey := propertyPrefix + "check-type"
	if properties[checkTypeKey] != "" {
		identifier.CheckType = properties[checkTypeKey]
	}

	checkSourceKey := propertyPrefix + "check-source"
	if properties[checkSourceKey] != "" {
		checkSourceString := properties[checkSourceKey]
		err := json.Unmarshal([]byte(checkSourceString), &identifier.CheckSource)
		if err != nil {
			return err
		}
	}

	container.identifier = identifier
	return nil
}

func (container *gardenWorkerContainer) IdentifierFromProperties() Identifier {
	return container.identifier
}

func (container *gardenWorkerContainer) heartbeat(pacemaker clock.Ticker) {
	defer container.heartbeating.Done()
	defer pacemaker.Stop()

	for {
		select {
		case <-pacemaker.C():
			container.SetProperty("keepalive", fmt.Sprintf("%d", container.clock.Now().Unix()))
		case <-container.stopHeartbeating:
			return
		}
	}
}
