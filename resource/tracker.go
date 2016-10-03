package resource

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type ResourceType string

type Session struct {
	ID        worker.Identifier
	Metadata  worker.Metadata
	Ephemeral bool
}

//go:generate counterfeiter . Tracker

type Tracker interface {
	Init(lager.Logger, Metadata, Session, ResourceType, atc.Tags, int, atc.ResourceTypes, worker.ImageFetchingDelegate) (Resource, error)
	InitWithSources(lager.Logger, Metadata, Session, ResourceType, atc.Tags, int, map[string]ArtifactSource, atc.ResourceTypes, worker.ImageFetchingDelegate) (Resource, []string, error)
}

//go:generate counterfeiter . Cache

type Cache interface {
	IsInitialized() (bool, error)
	Initialize() error
	Volume() worker.Volume
}

type Metadata interface {
	Env() []string
}

type tracker struct {
	workerClient worker.Client
}

type trackerFactory struct{}

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(client worker.Client) Tracker
}

func NewTrackerFactory() TrackerFactory {
	return &trackerFactory{}
}

func (factory *trackerFactory) TrackerFor(client worker.Client) Tracker {
	return &tracker{
		workerClient: client,
	}
}

func (tracker *tracker) InitWithSources(
	logger lager.Logger,
	metadata Metadata,
	session Session,
	typ ResourceType,
	tags atc.Tags,
	teamID int,
	sources map[string]ArtifactSource,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, []string, error) {
	logger = logger.Session("init-with-sources")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := tracker.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err, lager.Data{"id": session.ID})
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

	resourceSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(typ),
			Privileged:   true,
		},
		Ephemeral: session.Ephemeral,
		Tags:      tags,
		TeamID:    teamID,
		Env:       metadata.Env(),
	}

	compatibleWorkers, err := tracker.workerClient.AllSatisfying(resourceSpec.WorkerSpec(), resourceTypes)
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

	resourceSpec.Inputs = mounts

	logger.Debug("tracker-init-with-resources-creating-container", lager.Data{"container-id": session.ID})

	container, err = chosenWorker.CreateTaskContainer(
		logger,
		nil,
		imageFetchingDelegate,
		session.ID,
		session.Metadata,
		resourceSpec,
		resourceTypes,
		map[string]string{},
	)
	if err != nil {
		logger.Error("failed-to-create-container", err)
		return nil, nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return NewResource(container), missingSources, nil
}

func (tracker *tracker) Init(
	logger lager.Logger,
	metadata Metadata,
	session Session,
	typ ResourceType,
	tags atc.Tags,
	teamID int,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {
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

	logger.Debug("tracker-init-creating-container", lager.Data{"container-id": session.ID})

	container, err = tracker.workerClient.CreateTaskContainer(
		logger,
		nil,
		imageFetchingDelegate,
		session.ID,
		session.Metadata,
		worker.ContainerSpec{
			ImageSpec: worker.ImageSpec{
				ResourceType: string(typ),
				Privileged:   true,
			},
			Ephemeral: session.Ephemeral,
			Tags:      tags,
			TeamID:    teamID,
			Env:       metadata.Env(),
		},
		resourceTypes,
		map[string]string{},
	)
	if err != nil {
		return nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return NewResource(container), nil
}
