package resource

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

func NewResourceFactory(workerClient worker.Client) ResourceFactory {
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

//TODO ~> workerClient.Resource()
func (f *resourceFactory) NewResource(
	logger lager.Logger,
	signals <-chan os.Signal,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec worker.ContainerSpec,
	resourceTypes creds.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {

	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

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
