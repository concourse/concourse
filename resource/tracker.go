package resource

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

type ResourceType string
type ContainerImage string

type Session struct {
	ID        worker.Identifier
	Metadata  worker.Metadata
	Ephemeral bool
}

//go:generate counterfeiter . TrackerDB

type TrackerDB interface {
	InsertVolume(data db.Volume) error
}

//go:generate counterfeiter . Tracker

type Tracker interface {
	Init(lager.Logger, Metadata, Session, ResourceType, atc.Tags) (Resource, error)
	InitWithCache(lager.Logger, Metadata, Session, ResourceType, atc.Tags, CacheIdentifier) (Resource, Cache, error)
	InitWithSources(lager.Logger, Metadata, Session, ResourceType, atc.Tags, map[string]ArtifactSource) (Resource, []string, error)
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
	db           TrackerDB
}

type TrackerFactory struct {
	DB TrackerDB
}

func (factory TrackerFactory) TrackerFor(client worker.Client) Tracker {
	return NewTracker(client, factory.DB)
}

func NewTracker(workerClient worker.Client, db TrackerDB) Tracker {
	return &tracker{
		workerClient: workerClient,
		db:           db,
	}
}

type VolumeMount struct {
	Volume    baggageclaim.Volume
	MountPath string
}

func (tracker *tracker) InitWithSources(
	logger lager.Logger,
	metadata Metadata,
	session Session,
	typ ResourceType,
	tags atc.Tags,
	sources map[string]ArtifactSource,
) (Resource, []string, error) {
	logger = logger.Session("init-with-sources")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := tracker.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err)
		return nil, nil, err
	}

	if found {
		logger.Debug("found-existing-container", lager.Data{"container": container.Handle()})

		missingNames := []string{}

		for name, _ := range sources {
			missingNames = append(missingNames, name)
		}

		return NewResource(container), missingNames, nil
	}

	resourceSpec := worker.ResourceTypeContainerSpec{
		Type:      string(typ),
		Ephemeral: session.Ephemeral,
		Tags:      tags,
		Env:       metadata.Env(),
	}

	compatibleWorkers, err := tracker.workerClient.AllSatisfying(resourceSpec.WorkerSpec(), nil)
	if err != nil {
		return nil, nil, err
	}

	// find the worker with the most volumes
	mounts := []worker.VolumeMount{}
	missingSources := []string{}
	var chosenWorker worker.Worker

	for _, w := range compatibleWorkers {
		candidateMounts := []worker.VolumeMount{}
		missing := []string{}

		for name, source := range sources {
			ourVolume, found, err := source.VolumeOn(w)
			if err != nil {
				return nil, nil, err
			}

			if found {
				candidateMounts = append(candidateMounts, worker.VolumeMount{
					Volume:    ourVolume,
					MountPath: ResourcesDir("put/" + name),
				})
			} else {
				missing = append(missing, name)
			}
		}

		if len(candidateMounts) >= len(mounts) {
			for _, mount := range mounts {
				mount.Volume.Release(nil)
			}

			mounts = candidateMounts
			missingSources = missing
			chosenWorker = w
		} else {
			for _, mount := range candidateMounts {
				mount.Volume.Release(nil)
			}
		}
	}

	resourceSpec.Mounts = mounts

	container, err = chosenWorker.CreateContainer(logger, nil, nil, session.ID, session.Metadata, resourceSpec)
	if err != nil {
		logger.Error("failed-to-create-container", err)
		return nil, nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	for _, mount := range mounts {
		mount.Volume.Release(nil)
	}

	return NewResource(container), missingSources, nil
}

func (tracker *tracker) Init(logger lager.Logger, metadata Metadata, session Session, typ ResourceType, tags atc.Tags) (Resource, error) {
	logger = logger.Session("init")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := tracker.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err)
		return nil, err
	}

	if found {
		logger.Debug("found-existing-container", lager.Data{"container": container.Handle()})
		return NewResource(container), nil
	}

	logger.Debug("creating-container")

	container, err = tracker.workerClient.CreateContainer(logger, nil, nil, session.ID, session.Metadata, worker.ResourceTypeContainerSpec{
		Type:      string(typ),
		Ephemeral: session.Ephemeral,
		Tags:      tags,
		Env:       metadata.Env(),
	})
	if err != nil {
		return nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return NewResource(container), nil
}

func (tracker *tracker) InitWithCache(logger lager.Logger, metadata Metadata, session Session, typ ResourceType, tags atc.Tags, cacheIdentifier CacheIdentifier) (Resource, Cache, error) {
	logger = logger.Session("init-with-cache")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := tracker.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err)
		return nil, nil, err
	}

	if found {
		logger.Debug("found-existing-container", lager.Data{"container": container.Handle()})

		var cache Cache

		volumes := container.Volumes()
		switch len(volumes) {
		case 0:
			logger.Debug("no-cache")
			cache = noopCache{}
		default:
			logger.Debug("found-cache")
			cache = volumeCache{volumes[0]}
		}

		return NewResource(container), cache, nil
	}

	logger.Debug("no-existing-container")

	resourceSpec := worker.WorkerSpec{
		ResourceType: string(typ),
		Tags:         tags,
	}

	chosenWorker, err := tracker.workerClient.Satisfying(resourceSpec, nil)
	if err != nil {
		logger.Info("no-workers-satisfying-spec", lager.Data{
			"error": err.Error(),
		})
		return nil, nil, err
	}

	vm, hasVM := chosenWorker.VolumeManager()
	if !hasVM {
		logger.Debug("creating-container-without-cache")

		container, err := chosenWorker.CreateContainer(logger, nil, nil, session.ID, session.Metadata, worker.ResourceTypeContainerSpec{
			Type:      string(typ),
			Ephemeral: session.Ephemeral,
			Tags:      tags,
			Env:       metadata.Env(),
		})
		if err != nil {
			logger.Error("failed-to-create-container", err)
			return nil, nil, err
		}

		logger.Info("created", lager.Data{"container": container.Handle()})

		return NewResource(container), noopCache{}, nil
	}

	cachedVolume, cacheFound, err := cacheIdentifier.FindOn(logger, vm)
	if err != nil {
		logger.Error("failed-to-look-for-cache", err)
		return nil, nil, err
	}

	if cacheFound {
		logger.Debug("found-cache", lager.Data{"volume": cachedVolume.Handle()})
	} else {
		logger.Debug("no-cache-found")

		cachedVolume, err = cacheIdentifier.CreateOn(logger, vm)
		if err != nil {
			return nil, nil, err
		}

		logger.Debug("new-cache", lager.Data{"volume": cachedVolume.Handle()})
	}

	ttl, _, err := cachedVolume.Expiration()
	if err != nil {
		return nil, nil, err
	}

	err = tracker.db.InsertVolume(db.Volume{
		WorkerName:       chosenWorker.Name(),
		TTL:              ttl,
		Handle:           cachedVolume.Handle(),
		VolumeIdentifier: cacheIdentifier.VolumeIdentifier(),
	})
	if err != nil {
		return nil, nil, err
	}

	defer cachedVolume.Release(nil)

	logger.Debug("creating-container-with-cache")

	container, err = chosenWorker.CreateContainer(logger, nil, nil, session.ID, session.Metadata, worker.ResourceTypeContainerSpec{
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
		logger.Error("failed-to-create-container", err)
		return nil, nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return NewResource(container), volumeCache{cachedVolume}, nil
}
