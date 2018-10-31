package resource

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker"
)

func NewResourceFactory(workerClient worker.Client) ResourceFactory {
	return &resourceFactory{
		workerClient: workerClient,
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
		resourceConfig db.ResourceConfig,
	) (Resource, error)
}

type resourceFactory struct {
	workerClient worker.Client
}

func (f *resourceFactory) NewResource(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec worker.ContainerSpec,
	resourceTypes creds.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	resourceConfig db.ResourceConfig,
) (Resource, error) {

	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

	container, err := f.workerClient.FindOrCreateContainer(
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

	// Run info script using the container
	// _, _ = NewUnversionedResource(container).Info(ctx)

	// If info script run correctly, set the resource to v2, if not check if error is script not found and if yes then set to v1
	// if err != nil {

	// 	return err
	// }
	// if err == ErrNotScript {
	// 	resource = v2.NewResourceV1(container)
	// } else if err != nil {
	// 	resource = v2.NewResourceV2(container)
	// }

	return v2.NewResource(container, v2.ResourceInfo{}, resourceConfig), nil
}
