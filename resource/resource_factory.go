package resource

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
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
		signals <-chan os.Signal,
		owner db.ContainerOwner,
		metadata db.ContainerMetadata,
		containerSpec worker.ContainerSpec,
		resourceTypes creds.VersionedResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (Resource, error)
}

type resourceFactory struct {
	workerClient worker.Client
}

func (f *resourceFactory) NewResource(
	logger lager.Logger,
	signals <-chan os.Signal,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec worker.ContainerSpec,
	resourceTypes creds.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {
	container, err := f.workerClient.FindOrCreateContainer(
		logger,
		signals,
		imageFetchingDelegate,
		owner,
		metadata,
		containerSpec,
		resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	return NewResourceForContainer(container), nil
}
