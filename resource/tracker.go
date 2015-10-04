package resource

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

type ResourceType string
type ContainerImage string

type Session struct {
	ID        worker.Identifier
	Ephemeral bool
}

//go:generate counterfeiter . Tracker

type Tracker interface {
	Init(lager.Logger, Metadata, Session, ResourceType, atc.Tags) (Resource, error)
	InitWithCache(lager.Logger, Metadata, Session, ResourceType, atc.Tags, CacheIdentifier) (Resource, Cache, error)
}

//go:generate counterfeiter . Cache

type Cache interface {
	IsInitialized() (bool, error)
	Initialize() error
}

type Metadata interface {
	Env() []string
}

type EmptyMetadata struct{}

func (m EmptyMetadata) Env() []string { return nil }

type tracker struct {
	workerClient worker.Client
}

type TrackerFactory struct{}

func (TrackerFactory) TrackerFor(client worker.Client) Tracker {
	return NewTracker(client)
}

func NewTracker(workerClient worker.Client) Tracker {
	return &tracker{
		workerClient: workerClient,
	}
}

type VolumeMount struct {
	Volume    baggageclaim.Volume
	MountPath string
}

func (tracker *tracker) Init(logger lager.Logger, metadata Metadata, session Session, typ ResourceType, tags atc.Tags) (Resource, error) {
	container, found, err := tracker.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		return nil, err
	}

	if !found {
		container, err = tracker.workerClient.CreateContainer(logger, session.ID, worker.ResourceTypeContainerSpec{
			Type:      string(typ),
			Ephemeral: session.Ephemeral,
			Tags:      tags,
			Env:       metadata.Env(),
		})
		if err != nil {
			return nil, err
		}
	}

	return NewResource(container), nil
}

func (tracker *tracker) InitWithCache(logger lager.Logger, metadata Metadata, session Session, typ ResourceType, tags atc.Tags, cacheIdentifier CacheIdentifier) (Resource, Cache, error) {
	container, found, err := tracker.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		return nil, nil, err
	}

	if found {
		var cache Cache

		volumes := container.Volumes()
		switch len(volumes) {
		case 0:
			cache = noopCache{}
		default:
			cache = volumeCache{volumes[0]}
		}

		return NewResource(container), cache, nil
	}

	resourceSpec := worker.WorkerSpec{
		ResourceType: string(typ),
		Tags:         tags,
	}

	chosenWorker, err := tracker.workerClient.Satisfying(resourceSpec)
	if err != nil {
		return nil, nil, err
	}

	vm, hasVM := chosenWorker.VolumeManager()
	if !hasVM {
		container, err := chosenWorker.CreateContainer(logger, session.ID, worker.ResourceTypeContainerSpec{
			Type:      string(typ),
			Ephemeral: session.Ephemeral,
			Tags:      tags,
			Env:       metadata.Env(),
		})
		if err != nil {
			return nil, nil, err
		}

		return NewResource(container), noopCache{}, nil
	}

	cachedVolume, cacheFound, err := cacheIdentifier.FindOn(logger, vm)
	if err != nil {
		return nil, nil, err
	}

	if cacheFound {
		logger.Info("found-cache", lager.Data{"handle": cachedVolume.Handle()})
	} else {
		logger.Debug("no-cache-found")

		cachedVolume, err = cacheIdentifier.CreateOn(logger, vm)
		if err != nil {
			return nil, nil, err
		}

		logger.Info("new-cache", lager.Data{"handle": cachedVolume.Handle()})
	}

	defer cachedVolume.Release()

	container, err = chosenWorker.CreateContainer(logger, session.ID, worker.ResourceTypeContainerSpec{
		Type:      string(typ),
		Ephemeral: session.Ephemeral,
		Tags:      tags,
		Env:       metadata.Env(),
		Cache: worker.VolumeMount{
			Volume:    cachedVolume,
			MountPath: ResourcesDir("get"),
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return NewResource(container), volumeCache{cachedVolume}, nil
}
