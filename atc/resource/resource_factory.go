package resource

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
)

func NewResourceFactory(worker worker.Worker) ResourceFactory {
	return &resourceFactory{
		worker: worker,
	}
}

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewResource(
		ctx context.Context,
		logger lager.Logger,
		owner db.ContainerOwner,
		metadata db.ContainerMetadata,
		containerSpec worker.ContainerSpec,
		resourceTypes creds.VersionedResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (Resource, error)
}

type resourceFactory struct {
	worker worker.Worker
}

func (f *resourceFactory) NewResource(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec worker.ContainerSpec,
	resourceTypes creds.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {
	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

	container, err := f.worker.FindOrCreateContainer(
		ctx,
		logger,
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
