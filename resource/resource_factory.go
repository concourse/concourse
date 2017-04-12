package resource

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . ResourceFactoryFactory

type ResourceFactoryFactory interface {
	FactoryFor(workerClient worker.Client) ResourceFactory
}

type resourceFactoryFactory struct{}

func NewResourceFactoryFactory() ResourceFactoryFactory {
	return &resourceFactoryFactory{}
}

func (f *resourceFactoryFactory) FactoryFor(workerClient worker.Client) ResourceFactory {
	return &resourceFactory{
		workerClient: workerClient,
	}
}

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewPutResource(
		logger lager.Logger,
		signals <-chan os.Signal,
		buildID int,
		planID atc.PlanID,
		metadata dbng.ContainerMetadata,
		containerSpec worker.ContainerSpec,
		resourceTypes atc.VersionedResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (Resource, error)

	NewCheckResource(
		logger lager.Logger,
		signals <-chan os.Signal,
		resourceUser dbng.ResourceUser,
		resourceType string,
		resourceSource atc.Source,
		metadata dbng.ContainerMetadata,
		resourceSpec worker.ContainerSpec,
		resourceTypes atc.VersionedResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (Resource, error)
}

type resourceFactory struct {
	workerClient worker.Client
}

func (f *resourceFactory) NewPutResource(
	logger lager.Logger,
	signals <-chan os.Signal,
	buildID int,
	planID atc.PlanID,
	metadata dbng.ContainerMetadata,
	containerSpec worker.ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {
	container, err := f.workerClient.FindOrCreateBuildContainer(
		logger,
		signals,
		imageFetchingDelegate,
		buildID,
		planID,
		metadata,
		containerSpec,
		resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	return NewResourceForContainer(container), nil
}

func (f *resourceFactory) NewCheckResource(
	logger lager.Logger,
	signals <-chan os.Signal,
	resourceUser dbng.ResourceUser,
	resourceType string,
	resourceSource atc.Source,
	metadata dbng.ContainerMetadata,
	resourceSpec worker.ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {
	container, err := f.workerClient.FindOrCreateResourceCheckContainer(
		logger,
		resourceUser,
		signals,
		imageFetchingDelegate,
		metadata,
		resourceSpec,
		resourceTypes,
		resourceType,
		resourceSource,
	)
	if err != nil {
		return nil, err
	}

	return NewResourceForContainer(container), nil
}
