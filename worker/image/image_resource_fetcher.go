package image

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
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
		resource.ResourceFactory,
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
	resourceFetcherFactory  resource.FetcherFactory
	dbResourceCacheFactory  db.ResourceCacheFactory
	dbResourceConfigFactory db.ResourceConfigFactory
	clock                   clock.Clock
}

func NewImageResourceFetcherFactory(
	resourceFetcherFactory resource.FetcherFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	clock clock.Clock,
) ImageResourceFetcherFactory {
	return &imageResourceFetcherFactory{
		resourceFetcherFactory:  resourceFetcherFactory,
		dbResourceCacheFactory:  dbResourceCacheFactory,
		dbResourceConfigFactory: dbResourceConfigFactory,
		clock: clock,
	}
}

func (f *imageResourceFetcherFactory) NewImageResourceFetcher(
	worker worker.Worker,
	resourceFactory resource.ResourceFactory,
	imageResource worker.ImageResource,
	version atc.Version,
	teamID int,
	customTypes creds.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) ImageResourceFetcher {
	return &imageResourceFetcher{
		resourceFetcher:         f.resourceFetcherFactory.FetcherFor(worker),
		resourceFactory:         resourceFactory,
		dbResourceCacheFactory:  f.dbResourceCacheFactory,
		dbResourceConfigFactory: f.dbResourceConfigFactory,
		clock: f.clock,

		worker:                worker,
		imageResource:         imageResource,
		version:               version,
		teamID:                teamID,
		customTypes:           customTypes,
		imageFetchingDelegate: imageFetchingDelegate,
	}
}

type imageResourceFetcher struct {
	worker                  worker.Worker
	resourceFetcher         resource.Fetcher
	resourceFactory         resource.ResourceFactory
	dbResourceCacheFactory  db.ResourceCacheFactory
	dbResourceConfigFactory db.ResourceConfigFactory
	clock                   clock.Clock

	imageResource         worker.ImageResource
	version               atc.Version
	teamID                int
	customTypes           creds.VersionedResourceTypes
	imageFetchingDelegate worker.ImageFetchingDelegate
	variables             creds.Variables
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
		db.NewImageGetContainerOwner(container),
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

	versionedSource, err := i.resourceFetcher.Fetch(
		ctx,
		logger.Session("init-image"),
		getSess,
		i.worker.Tags(),
		i.teamID,
		i.customTypes,
		resourceInstance,
		resource.EmptyMetadata{},
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

	checkResourceType, err := i.resourceFactory.NewResource(
		ctx,
		logger,
		db.NewImageCheckContainerOwner(container),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		worker.ContainerSpec{
			ImageSpec: worker.ImageSpec{
				ResourceType: resourceType.Name,
			},
			Tags:   i.worker.Tags(),
			TeamID: i.teamID,
		}, i.customTypes,
		worker.NoopImageFetchingDelegate{},
	)
	if err != nil {
		return err
	}

	source, err := resourceType.Source.Evaluate()
	if err != nil {
		return err
	}

	versions, err := checkResourceType.Check(source, nil)
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
		Tags:   i.worker.Tags(),
		TeamID: i.teamID,
	}

	source, err := i.imageResource.Source.Evaluate()
	if err != nil {
		return nil, err
	}

	checkingResource, err := i.resourceFactory.NewResource(
		ctx,
		logger,
		db.NewImageCheckContainerOwner(container),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		resourceSpec,
		i.customTypes,
		i.imageFetchingDelegate,
	)
	if err != nil {
		return nil, err
	}

	versions, err := checkingResource.Check(source, nil)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, ErrImageUnavailable
	}

	return versions[0], nil
}

type leaseID struct {
	Type       resource.ResourceType `json:"type"`
	Version    atc.Version           `json:"version"`
	Source     atc.Source            `json:"source"`
	WorkerName string                `json:"worker_name"`
}

type readCloser struct {
	io.Reader
	io.Closer
}
