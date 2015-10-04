package worker

import (
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")
var ErrIncompatiblePlatform = errors.New("incompatible platform")
var ErrMismatchedTags = errors.New("mismatched tags")

const containerKeepalive = 30 * time.Second
const containerTTL = 5 * time.Minute

const inputVolumeTTL = 60 * 5

const ephemeralPropertyName = "concourse:ephemeral"
const volumePropertyName = "concourse:volumes"

var trackedContainers = expvar.NewInt("TrackedContainers")

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int

	Description() string

	VolumeManager() (baggageclaim.Client, bool)
}

//go:generate counterfeiter . GardenWorkerDB

type GardenWorkerDB interface {
	CreateContainerInfo(db.ContainerInfo, time.Duration) error
	UpdateExpiresAtOnContainerInfo(handle string, ttl time.Duration) error
}

type gardenWorker struct {
	gardenClient       garden.Client
	baggageclaimClient baggageclaim.Client
	db                 GardenWorkerDB
	provider           WorkerProvider

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
	db GardenWorkerDB,
	provider WorkerProvider,
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
		db:                 db,
		provider:           provider,
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

func (worker *gardenWorker) CreateContainer(logger lager.Logger, id Identifier, spec ContainerSpec) (Container, error) {
	gardenSpec := garden.ContainerSpec{
		Properties: garden.Properties{},
	}

	var volumeHandles []string

dance:
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		gardenSpec.Privileged = true

		gardenSpec.Env = s.Env

		if s.Ephemeral {
			gardenSpec.Properties[ephemeralPropertyName] = "true"
		}

		if s.Cache.Volume != nil && s.Cache.MountPath != "" {
			gardenSpec.BindMounts = []garden.BindMount{
				{
					SrcPath: s.Cache.Volume.Path(),
					DstPath: s.Cache.MountPath,
					Mode:    garden.BindMountModeRW,
				},
			}

			volumeHandles = append(volumeHandles, s.Cache.Volume.Handle())
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

		baseVolumes, baseBindMounts, err := worker.createGardenWorkaroundVolumes(logger, s)
		if err != nil {
			return nil, err
		}

		for _, volume := range baseVolumes {
			// release *after* container creation
			defer volume.Release()
			volumeHandles = append(volumeHandles, volume.Handle())
		}

		gardenSpec.BindMounts = append(gardenSpec.BindMounts, baseBindMounts...)

		for _, input := range s.Inputs {
			cow, err := worker.baggageclaimClient.CreateVolume(logger, baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{
					Parent: input.Volume,
				},
				Privileged:   s.Privileged,
				TTLInSeconds: inputVolumeTTL,
			})
			if err != nil {
				return nil, err
			}

			// release *after* container creation
			defer cow.Release()

			gardenSpec.BindMounts = append(gardenSpec.BindMounts, garden.BindMount{
				SrcPath: cow.Path(),
				DstPath: input.MountPath,
				Mode:    garden.BindMountModeRW,
			})

			volumeHandles = append(volumeHandles, cow.Handle())
		}

	default:
		return nil, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}

	if len(volumeHandles) > 0 {
		volumesJSON, err := json.Marshal(volumeHandles)
		if err != nil {
			return nil, err
		}

		gardenSpec.Properties[volumePropertyName] = string(volumesJSON)
	}

	gardenContainer, err := worker.gardenClient.Create(gardenSpec)
	if err != nil {
		return nil, err
	}

	idWithWorker := db.ContainerIdentifier(id)
	idWithWorker.WorkerName = worker.name

	err = worker.db.CreateContainerInfo(
		db.ContainerInfo{
			ContainerIdentifier: idWithWorker,
			Handle:              gardenContainer.Handle(),
		}, containerTTL)
	if err != nil {
		return nil, err
	}

	return newGardenWorkerContainer(logger, gardenContainer, worker.gardenClient, worker.baggageclaimClient, worker.db, worker.clock)
}

func (worker *gardenWorker) FindContainerForIdentifier(logger lager.Logger, id Identifier) (Container, bool, error) {
	containerInfo, found, err := worker.provider.FindContainerInfoForIdentifier(id)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, found, nil
	}

	container, found, err := worker.LookupContainer(logger, containerInfo.Handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		logger.Info("reaping-container-not-found-on-worker", lager.Data{
			"container-handle": containerInfo.Handle,
			"worker-name":      containerInfo.WorkerName,
		})

		err := worker.provider.ReapContainer(containerInfo.Handle)
		if err != nil {
			return nil, false, err
		}

		return nil, false, err
	}

	return container, found, nil
}

func (worker *gardenWorker) LookupContainer(logger lager.Logger, handle string) (Container, bool, error) {
	gardenContainer, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Info("container-not-found")
			return nil, false, nil
		}

		logger.Error("failed-to-lookup-on-garden", err)
		return nil, false, err
	}

	container, err := newGardenWorkerContainer(logger, gardenContainer, worker.gardenClient, worker.baggageclaimClient, worker.db, worker.clock)
	if err != nil {
		logger.Error("failed-to-construct-container", err)
		return nil, false, err
	}

	return container, true, nil
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

func (worker *gardenWorker) createGardenWorkaroundVolumes(
	logger lager.Logger,
	spec TaskContainerSpec,
) ([]baggageclaim.Volume, []garden.BindMount, error) {
	existingMounts := map[string]bool{}

	volumes := []baggageclaim.Volume{}
	bindMounts := []garden.BindMount{}

	for _, m := range spec.Inputs {
		for _, dir := range pathSegments(m.MountPath) {
			if existingMounts[dir] {
				continue
			}

			existingMounts[dir] = true

			volume, err := worker.baggageclaimClient.CreateVolume(
				logger.Session("workaround"),
				baggageclaim.VolumeSpec{
					Privileged:   spec.Privileged,
					Strategy:     baggageclaim.EmptyStrategy{},
					TTLInSeconds: inputVolumeTTL,
				},
			)
			if err != nil {
				for _, v := range volumes {
					// prevent leaking previously created volumes
					v.Release()
				}

				return nil, nil, err
			}

			volumes = append(volumes, volume)
			bindMounts = append(bindMounts, garden.BindMount{
				SrcPath: volume.Path(),
				DstPath: dir,
				Mode:    garden.BindMountModeRW,
			})
		}
	}

	return volumes, bindMounts, nil
}

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client
	db           GardenWorkerDB

	volumes []baggageclaim.Volume

	clock clock.Clock

	stopHeartbeating chan struct{}
	heartbeating     *sync.WaitGroup

	releaseOnce sync.Once
}

func newGardenWorkerContainer(
	logger lager.Logger,
	container garden.Container,
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	db GardenWorkerDB,
	clock clock.Clock,
) (Container, error) {
	workerContainer := &gardenWorkerContainer{
		Container: container,

		gardenClient: gardenClient,
		db:           db,

		clock: clock,

		heartbeating:     new(sync.WaitGroup),
		stopHeartbeating: make(chan struct{}),
	}

	workerContainer.heartbeating.Add(1)
	go workerContainer.heartbeat(logger, clock.NewTicker(containerKeepalive))

	trackedContainers.Add(1)
	metric.TrackedContainers.Inc()

	properties, err := workerContainer.Properties()
	if err != nil {
		workerContainer.Release()
		return nil, err
	}

	err = workerContainer.initializeVolumes(logger, properties, baggageclaimClient)
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

		for _, v := range container.volumes {
			v.Release()
		}
	})
}

func (container *gardenWorkerContainer) Volumes() []baggageclaim.Volume {
	return container.volumes
}

func (container *gardenWorkerContainer) initializeVolumes(
	logger lager.Logger,
	properties garden.Properties,
	baggageclaimClient baggageclaim.Client,
) error {
	if baggageclaimClient == nil {
		return nil
	}

	handlesJSON, found := properties[volumePropertyName]
	if !found {
		container.volumes = []baggageclaim.Volume{}
		return nil
	}

	var handles []string
	err := json.Unmarshal([]byte(handlesJSON), &handles)
	if err != nil {
		return err
	}

	volumes := []baggageclaim.Volume{}
	for _, h := range handles {
		volume, err := baggageclaimClient.LookupVolume(logger, h)
		if err != nil {
			return err
		}

		volumes = append(volumes, volume)
	}

	container.volumes = volumes

	return nil
}

func (container *gardenWorkerContainer) heartbeat(logger lager.Logger, pacemaker clock.Ticker) {
	defer container.heartbeating.Done()
	defer pacemaker.Stop()

	logger = logger.Session("heartbeating")

	logger.Debug("start")
	defer logger.Debug("done")

	for {
		select {
		case <-pacemaker.C():
			err := container.db.UpdateExpiresAtOnContainerInfo(container.Handle(), containerTTL)
			if err != nil {
				logger.Error("failed-to-heartbeat-to-db", err)
			}

			err = container.SetProperty("keepalive", fmt.Sprintf("%d", container.clock.Now().Unix()))
			if err != nil {
				logger.Error("failed-to-heartbeat-to-container", err)
			}

		case <-container.stopHeartbeating:
			return
		}
	}
}

func pathSegments(p string) []string {
	segs := []string{}

	for dir := path.Dir(p); dir != "/"; dir = path.Dir(dir) {
		segs = append(segs, dir)
	}

	sort.Sort(sort.StringSlice(segs))

	return segs
}
