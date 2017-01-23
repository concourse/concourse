package resource

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
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
	NewResource(
		logger lager.Logger,
		id worker.Identifier,
		metadata worker.Metadata,
		resourceSpec worker.ContainerSpec,
		resourceTypes atc.ResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
		resourceSources map[string]worker.ArtifactSource,
	) (Resource, []string, error)

	NewCheckResource(
		logger lager.Logger,
		id worker.Identifier,
		metadata worker.Metadata,
		resourceSpec worker.ContainerSpec,
		resourceTypes atc.ResourceTypes,
	) (Resource, error)

	NewCheckResourceForResourceType(
		logger lager.Logger,
		id worker.Identifier,
		metadata worker.Metadata,
		resourceSpec worker.ContainerSpec,
		resourceTypes atc.ResourceTypes,
	) (Resource, error)
}

type resourceFactory struct {
	workerClient worker.Client
}

func (f *resourceFactory) NewResource(
	logger lager.Logger,
	id worker.Identifier,
	metadata worker.Metadata,
	resourceSpec worker.ContainerSpec,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	resourceSources map[string]worker.ArtifactSource,
) (Resource, []string, error) {
	container, missingSourceNames, err := f.workerClient.FindOrCreateContainerForIdentifier(
		logger,
		id,
		metadata,
		resourceSpec,
		resourceTypes,
		imageFetchingDelegate,
		resourceSources,
	)
	if err != nil {
		return nil, nil, err
	}

	return NewResourceForContainer(container), missingSourceNames, nil
}

func (f *resourceFactory) NewCheckResource(
	logger lager.Logger,
	id worker.Identifier,
	metadata worker.Metadata,
	resourceSpec worker.ContainerSpec,
	resourceTypes atc.ResourceTypes,
) (Resource, error) {
	container, err := f.workerClient.FindOrCreateResourceCheckContainer(
		logger,
		nil,
		worker.NoopImageFetchingDelegate{},
		id,
		metadata,
		resourceSpec,
		resourceTypes,
		id.CheckType,
		id.CheckSource,
	)
	if err != nil {
		return nil, err
	}

	return NewResourceForContainer(container), nil
}

func (f *resourceFactory) NewCheckResourceForResourceType(
	logger lager.Logger,
	id worker.Identifier,
	metadata worker.Metadata,
	resourceSpec worker.ContainerSpec,
	resourceTypes atc.ResourceTypes,
) (Resource, error) {
	container, err := f.workerClient.FindOrCreateResourceTypeCheckContainer(
		logger,
		nil,
		worker.NoopImageFetchingDelegate{},
		id,
		metadata,
		resourceSpec,
		resourceTypes,
		id.CheckType,
		id.CheckSource,
	)
	if err != nil {
		return nil, err
	}

	return NewResourceForContainer(container), nil
}
