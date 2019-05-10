package image

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/DataDog/zstd"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

const ImageMetadataFile = "metadata.json"

// ErrImageUnavailable is returned when a task's configured image resource
// has no versions.
var ErrImageUnavailable = errors.New("no versions of image available")

var ErrImageGetDidNotProduceVolume = errors.New("fetching the image did not produce a volume")

//go:generate counterfeiter . ImageResourceFetcherFactory

type ImageResourceFetcherFactory interface {
	NewImageResourceFetcher(
		worker.Worker,
		worker.ImageResource,
		atc.Version,
		int,
		creds.VersionedResourceTypes,
		worker.ImageFetchingDelegate,
	) ImageResourceFetcher
}

//go:generate counterfeiter . ImageResourceFetcher

type ImageResourceFetcher interface {
	Fetch(
		ctx context.Context,
		logger lager.Logger,
		container db.CreatingContainer,
		privileged bool,
	) (worker.Volume, io.ReadCloser, atc.Version, error)
}

type imageResourceFetcherFactory struct {
	dbResourceCacheFactory  db.ResourceCacheFactory
	dbResourceConfigFactory db.ResourceConfigFactory
	resourceFetcher         resource.Fetcher
	resourceFactory         resource.ResourceFactory
}

func NewImageResourceFetcherFactory(
	dbResourceCacheFactory db.ResourceCacheFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
) ImageResourceFetcherFactory {
	return &imageResourceFetcherFactory{
		dbResourceCacheFactory:  dbResourceCacheFactory,
		dbResourceConfigFactory: dbResourceConfigFactory,
		resourceFetcher:         resourceFetcher,
		resourceFactory:         resourceFactory,
	}
}

func (f *imageResourceFetcherFactory) NewImageResourceFetcher(
	worker worker.Worker,
	imageResource worker.ImageResource,
	version atc.Version,
	teamID int,
	customTypes creds.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) ImageResourceFetcher {
	return &imageResourceFetcher{
		worker:                  worker,
		resourceFactory:         f.resourceFactory,
		resourceFetcher:         f.resourceFetcher,
		dbResourceCacheFactory:  f.dbResourceCacheFactory,
		dbResourceConfigFactory: f.dbResourceConfigFactory,

		imageResource:         imageResource,
		version:               version,
		teamID:                teamID,
		customTypes:           customTypes,
		imageFetchingDelegate: imageFetchingDelegate,
	}
}

type imageResourceFetcher struct {
	worker                  worker.Worker
	resourceFactory         resource.ResourceFactory
	resourceFetcher         resource.Fetcher
	dbResourceCacheFactory  db.ResourceCacheFactory
	dbResourceConfigFactory db.ResourceConfigFactory

	imageResource         worker.ImageResource
	version               atc.Version
	teamID                int
	customTypes           creds.VersionedResourceTypes
	imageFetchingDelegate worker.ImageFetchingDelegate
}

func (i *imageResourceFetcher) Fetch(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
	privileged bool,
) (worker.Volume, io.ReadCloser, atc.Version, error) {
	version := i.version
	if version == nil {
		var err error
		version, err = i.getLatestVersion(ctx, logger, container)
		if err != nil {
			logger.Error("failed-to-get-latest-image-version", err)
			return nil, nil, nil, err
		}
	}

	source, err := i.imageResource.Source.Evaluate()
	if err != nil {
		return nil, nil, nil, err
	}

	var params atc.Params
	if i.imageResource.Params != nil {
		params = *i.imageResource.Params
	}

	resourceCache, err := i.dbResourceCacheFactory.FindOrCreateResourceCache(
		logger,
		db.ForContainer(container.ID()),
		i.imageResource.Type,
		version,
		source,
		params,
		i.customTypes,
	)
	if err != nil {
		logger.Error("failed-to-create-resource-cache", err)
		return nil, nil, nil, err
	}

	resourceInstance := resource.NewResourceInstance(
		resource.ResourceType(i.imageResource.Type),
		version,
		source,
		params,
		i.customTypes,
		resourceCache,
		db.NewImageGetContainerOwner(container, i.teamID),
	)

	err = i.imageFetchingDelegate.ImageVersionDetermined(resourceCache)
	if err != nil {
		return nil, nil, nil, err
	}

	getSess := resource.Session{
		Metadata: db.ContainerMetadata{
			Type: db.ContainerTypeGet,
		},
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(resourceInstance.ResourceType()),
		},
		TeamID: i.teamID,
	}

	// The random placement strategy is not really used because the image
	// resource will always find the same worker as the container that owns it
	versionedSource, err := i.resourceFetcher.Fetch(
		ctx,
		logger.Session("init-image"),
		getSess,
		i.worker,
		containerSpec,
		i.customTypes,
		resourceInstance,
		i.imageFetchingDelegate,
	)
	if err != nil {
		logger.Error("failed-to-fetch-image", err)
		return nil, nil, nil, err
	}

	volume := versionedSource.Volume()
	if volume == nil {
		return nil, nil, nil, ErrImageGetDidNotProduceVolume
	}

	reader, err := versionedSource.StreamOut(ImageMetadataFile)
	if err != nil {
		return nil, nil, nil, err
	}

	zstdReader := zstd.NewReader(reader)
	tarReader := tar.NewReader(zstdReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read file \"%s\" from tar", ImageMetadataFile)
	}

	releasingReader := &readCloser{
		Reader: tarReader,
		Closer: reader,
	}

	return volume, releasingReader, version, nil
}

func (i *imageResourceFetcher) ensureVersionOfType(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
	resourceType creds.VersionedResourceType,
) error {
	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: resourceType.Name,
		},
		TeamID: i.teamID,
		BindMounts: []worker.BindMountSource{
			&worker.CertsVolumeMount{Logger: logger},
		},
	}

	resourceTypeContainer, err := i.worker.FindOrCreateContainer(
		ctx,
		logger,
		worker.NoopImageFetchingDelegate{},
		db.NewImageCheckContainerOwner(container, i.teamID),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		containerSpec,
		i.customTypes,
	)
	if err != nil {
		return err
	}

	source, err := resourceType.Source.Evaluate()
	if err != nil {
		return err
	}

	checkResourceType := i.resourceFactory.NewResourceForContainer(resourceTypeContainer)
	versions, err := checkResourceType.Check(context.TODO(), source, nil)
	if err != nil {
		return err
	}

	if len(versions) == 0 {
		return ErrImageUnavailable
	}

	resourceType.Version = versions[0]

	i.customTypes = append(i.customTypes.Without(resourceType.Name), resourceType)

	return nil
}

func (i *imageResourceFetcher) getLatestVersion(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
) (atc.Version, error) {

	resourceType, found := i.customTypes.Lookup(i.imageResource.Type)
	if found && resourceType.Version == nil {
		err := i.ensureVersionOfType(ctx, logger, container, resourceType)
		if err != nil {
			return nil, err
		}
	}

	resourceSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: i.imageResource.Type,
		},
		TeamID: i.teamID,
		BindMounts: []worker.BindMountSource{
			&worker.CertsVolumeMount{Logger: logger},
		},
	}

	source, err := i.imageResource.Source.Evaluate()
	if err != nil {
		return nil, err
	}

	imageContainer, err := i.worker.FindOrCreateContainer(
		ctx,
		logger,
		i.imageFetchingDelegate,
		db.NewImageCheckContainerOwner(container, i.teamID),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		resourceSpec,
		i.customTypes,
	)
	if err != nil {
		return nil, err
	}

	checkingResource := i.resourceFactory.NewResourceForContainer(imageContainer)
	versions, err := checkingResource.Check(context.TODO(), source, nil)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, ErrImageUnavailable
	}

	return versions[0], nil
}

type readCloser struct {
	io.Reader
	io.Closer
}
