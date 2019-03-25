package image

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
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

var ErrNoSpaceSpecified = errors.New("no space specified and no default space available")

//go:generate counterfeiter . ImageResourceFetcherFactory

type ImageResourceFetcherFactory interface {
	NewImageResourceFetcher(
		worker.Worker,
		worker.ImageResource,
		atc.Version,
		atc.Space,
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
	dbResourceCacheFactory db.ResourceCacheFactory
	resourceFetcher        resource.Fetcher
	resourceFactory        resource.ResourceFactory
}

func NewImageResourceFetcherFactory(
	dbResourceCacheFactory db.ResourceCacheFactory,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
) ImageResourceFetcherFactory {
	return &imageResourceFetcherFactory{
		dbResourceCacheFactory: dbResourceCacheFactory,
		resourceFetcher:        resourceFetcher,
		resourceFactory:        resourceFactory,
	}
}

func (f *imageResourceFetcherFactory) NewImageResourceFetcher(
	worker worker.Worker,
	imageResource worker.ImageResource,
	version atc.Version,
	defaultSpace atc.Space,
	teamID int,
	customTypes creds.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) ImageResourceFetcher {
	return &imageResourceFetcher{
		worker:                 worker,
		resourceFactory:        f.resourceFactory,
		resourceFetcher:        f.resourceFetcher,
		dbResourceCacheFactory: f.dbResourceCacheFactory,

		imageResource:         imageResource,
		version:               version,
		defaultSpace:          defaultSpace,
		teamID:                teamID,
		customTypes:           customTypes,
		imageFetchingDelegate: imageFetchingDelegate,
	}
}

type imageResourceFetcher struct {
	worker                 worker.Worker
	resourceFactory        resource.ResourceFactory
	resourceFetcher        resource.Fetcher
	dbResourceCacheFactory db.ResourceCacheFactory

	imageResource         worker.ImageResource
	version               atc.Version
	defaultSpace          atc.Space
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
	defaultSpace := i.defaultSpace
	if version == nil || defaultSpace == "" {
		var err error
		defaultSpace, version, err = i.getLatestVersion(ctx, logger, container)
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

	// XXX: Fix find or create resource cache to use space
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
		defaultSpace,
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
	volume, err := i.resourceFetcher.Fetch(
		ctx,
		logger.Session("init-image"),
		getSess,
		NewGetEventHandler(),
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

	if volume == nil {
		return nil, nil, nil, ErrImageGetDidNotProduceVolume
	}

	reader, err := volume.StreamOut(ImageMetadataFile)
	if err != nil {
		return nil, nil, nil, err
	}

	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, nil, nil, err
	}

	tarReader := tar.NewReader(gzReader)

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

	latestVersions := map[atc.Space]atc.Version{}
	eventHandler := CheckEventHandler{SavedLatestVersions: latestVersions}
	checkResourceType, err := i.resourceFactory.NewResourceForContainer(context.TODO(), resourceTypeContainer)
	if err != nil {
		return err
	}

	err = checkResourceType.Check(context.TODO(), &eventHandler, source, nil)
	if err != nil {
		return err
	}

	var defaultSpace atc.Space
	var version atc.Version
	var ok bool

	if resourceType.Space == "" {
		if eventHandler.SavedDefaultSpace == "" {
			return ErrNoSpaceSpecified
		}
		defaultSpace = eventHandler.SavedDefaultSpace
	} else {
		defaultSpace = resourceType.Space
	}

	if version, ok = eventHandler.SavedLatestVersions[defaultSpace]; !ok {
		return ErrImageUnavailable
	}

	resourceType.Version = version
	resourceType.Space = defaultSpace
	i.customTypes = append(i.customTypes.Without(resourceType.Name), resourceType)

	return nil
}

func (i *imageResourceFetcher) getLatestVersion(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
) (atc.Space, atc.Version, error) {
	resourceType, found := i.customTypes.Lookup(i.imageResource.Type)
	if found && resourceType.Version == nil {
		err := i.ensureVersionOfType(ctx, logger, container, resourceType)
		if err != nil {
			return "", nil, err
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
		return "", nil, err
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
		return "", nil, err
	}

	latestVersions := map[atc.Space]atc.Version{}
	eventHandler := CheckEventHandler{SavedLatestVersions: latestVersions}
	checkingResource, err := i.resourceFactory.NewResourceForContainer(context.TODO(), imageContainer)
	if err != nil {
		return "", nil, err
	}

	err = checkingResource.Check(context.TODO(), &eventHandler, source, nil)
	if err != nil {
		return "", nil, err
	}

	var defaultSpace atc.Space
	var version atc.Version
	var ok bool

	if i.defaultSpace == "" {
		if eventHandler.SavedDefaultSpace == "" {
			return "", nil, ErrNoSpaceSpecified
		}
		defaultSpace = eventHandler.SavedDefaultSpace
	} else {
		defaultSpace = i.defaultSpace
	}

	if version, ok = eventHandler.SavedLatestVersions[defaultSpace]; !ok {
		return "", nil, ErrImageUnavailable
	}

	return defaultSpace, version, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}
