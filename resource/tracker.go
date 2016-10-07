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
