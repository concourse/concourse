package resource

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . FetchContainerCreatorFactory

type FetchContainerCreatorFactory interface {
	NewFetchContainerCreator(
		logger lager.Logger,
		resourceTypes atc.ResourceTypes,
		tags atc.Tags,
		teamID int,
		session Session,
		metadata Metadata,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) FetchContainerCreator
}

//go:generate counterfeiter . FetchContainerCreator

type FetchContainerCreator interface {
	CreateWithVolume(string, worker.Volume, worker.Worker) (worker.Container, error)
}

type fetchContainerCreator struct {
	logger                lager.Logger
	resourceTypes         atc.ResourceTypes
	tags                  atc.Tags
	teamID                int
	session               Session
	metadata              Metadata
	imageFetchingDelegate worker.ImageFetchingDelegate
}

type fetchContainerCreatorFactory struct{}

func NewFetchContainerCreatorFactory() FetchContainerCreatorFactory {
	return fetchContainerCreatorFactory{}
}

func (f fetchContainerCreatorFactory) NewFetchContainerCreator(
	logger lager.Logger,
	resourceTypes atc.ResourceTypes,
	tags atc.Tags,
	teamID int,
	session Session,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) FetchContainerCreator {
	return &fetchContainerCreator{
		logger:                logger,
		resourceTypes:         resourceTypes,
		tags:                  tags,
		teamID:                teamID,
		session:               session,
		metadata:              metadata,
		imageFetchingDelegate: imageFetchingDelegate,
	}
}

func (c *fetchContainerCreator) CreateWithVolume(resourceType string, volume worker.Volume, chosenWorker worker.Worker) (worker.Container, error) {
	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: resourceType,
			Privileged:   true,
		},
		Ephemeral: c.session.Ephemeral,
		Tags:      c.tags,
		TeamID:    c.teamID,
		Env:       c.metadata.Env(),
		Outputs: []worker.VolumeMount{
			{
				Volume:    volume,
				MountPath: ResourcesDir("get"),
			},
		},
	}

	return chosenWorker.CreateContainer(
		c.logger,
		nil,
		c.imageFetchingDelegate,
		c.session.ID,
		c.session.Metadata,
		containerSpec,
		c.resourceTypes,
	)
}
