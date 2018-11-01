package resource

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker"
)

type ErrUnknownResourceVersion struct {
	version string
}

func (e ErrUnknownResourceVersion) Error() string {
	return fmt.Sprintf("unknown resource version: %s", e.version)
}

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

	resourceInfo, err := NewUnversionedResource(container).Info(ctx)

	var resource Resource
	if err == nil {
		if resourceInfo.Artifacts.APIVersion == "2.0" {
			resource = v2.NewResource(container, resourceInfo, resourceConfig)
		} else {
			return nil, ErrUnknownResourceVersion{resourceInfo.Artifacts.APIVersion}
		}
	} else if typeErr, ok := err.(garden.ExecutableNotFoundError); ok {
		if err != nil {
			return nil, typeErr
		}

		resource = v2.NewV1Adapter(container, resourceConfig)
	} else if err != nil {
		return nil, err
	}

	return resource, nil
}
