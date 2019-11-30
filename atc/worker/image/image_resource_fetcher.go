package image

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc/resource"

	"github.com/hashicorp/go-multierror"

	"code.cloudfoundry.org/lager"
	"github.com/DataDog/zstd"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

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
		atc.VersionedResourceTypes,
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
	resourceFactory         resource.ResourceFactory
	dbResourceCacheFactory  db.ResourceCacheFactory
	dbResourceConfigFactory db.ResourceConfigFactory
	resourceFetcher         worker.Fetcher
}

func NewImageResourceFetcherFactory(
	resourceFactory resource.ResourceFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	resourceFetcher worker.Fetcher,
) ImageResourceFetcherFactory {
	return &imageResourceFetcherFactory{
		resourceFactory:         resourceFactory,
		dbResourceCacheFactory:  dbResourceCacheFactory,
		dbResourceConfigFactory: dbResourceConfigFactory,
		resourceFetcher:         resourceFetcher,
	}
}

func (f *imageResourceFetcherFactory) NewImageResourceFetcher(
	worker worker.Worker,
	imageResource worker.ImageResource,
	version atc.Version,
	teamID int,
	customTypes atc.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) ImageResourceFetcher {
	return &imageResourceFetcher{
		worker:                  worker,
		resourceFetcher:         f.resourceFetcher,
		resourceFactory:         f.resourceFactory,
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
	resourceFetcher         worker.Fetcher
	resourceFactory         resource.ResourceFactory
	dbResourceCacheFactory  db.ResourceCacheFactory
	dbResourceConfigFactory db.ResourceConfigFactory

	imageResource         worker.ImageResource
	version               atc.Version
	teamID                int
	customTypes           atc.VersionedResourceTypes
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

	params := i.imageResource.Params

	resourceCache, err := i.dbResourceCacheFactory.FindOrCreateResourceCache(
		db.ForContainer(container.ID()),
		i.imageResource.Type,
		version,
		i.imageResource.Source,
		params,
		i.customTypes,
	)
	if err != nil {
		logger.Error("failed-to-create-resource-cache", err)
		return nil, nil, nil, err
	}

	err = i.imageFetchingDelegate.ImageVersionDetermined(resourceCache)
	if err != nil {
		return nil, nil, nil, err
	}

	containerMetadata := db.ContainerMetadata{
		Type: db.ContainerTypeGet,
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: i.imageResource.Type,
		},
		TeamID: i.teamID,
	}

	processSpec := runtime.ProcessSpec{
		Path:         "/opt/resource/in",
		Args:         []string{resource.ResourcesDir("get")},
		StdoutWriter: i.imageFetchingDelegate.Stdout(),
		StderrWriter: i.imageFetchingDelegate.Stderr(),
	}
	res := i.resourceFactory.NewResource(
		i.imageResource.Source,
		params,
		version,
	)

	sign, err := res.Signature()
	if err != nil {
		return nil, nil, nil, err
	}

	lockName := lockName(sign, i.worker.Name())
	imageFetcherSpec := worker.ImageFetcherSpec{
		ResourceTypes: i.customTypes,
		Delegate:      i.imageFetchingDelegate,
	}

	_, volume, err := i.resourceFetcher.Fetch(
		ctx,
		logger.Session("init-image"),
		containerMetadata,
		i.worker,
		containerSpec,
		processSpec,
		res,
		db.NewImageGetContainerOwner(container, i.teamID),
		imageFetcherSpec,
		resourceCache,
		lockName,
	)

	if err != nil {
		logger.Error("failed-to-fetch-image", err)
		return nil, nil, nil, err
	}

	if volume == nil {
		return nil, nil, nil, ErrImageGetDidNotProduceVolume
	}

	reader, err := volume.StreamOut(ctx, ImageMetadataFile)
	if err != nil {
		return nil, nil, nil, err
	}

	zstdReader := zstd.NewReader(reader)
	tarReader := tar.NewReader(zstdReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read file \"%s\" from tar", ImageMetadataFile)
	}

	releasingReader := &fileReadMultiCloser{
		reader: tarReader,
		closers: []io.Closer{
			reader,
			zstdReader,
		},
	}

	return volume, releasingReader, version, nil
}

func (i *imageResourceFetcher) ensureVersionOfType(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
	resourceType atc.VersionedResourceType,
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

	owner := db.NewImageCheckContainerOwner(container, i.teamID)

	resourceTypeContainer, err := i.worker.FindOrCreateContainer(
		ctx,
		logger,
		worker.NoopImageFetchingDelegate{},
		owner,
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		containerSpec,
		i.customTypes,
	)
	if err != nil {
		return err
	}

	processSpec := runtime.ProcessSpec{
		Path: "/opt/resource/check",
	}
	checkResourceType := i.resourceFactory.NewResource(resourceType.Source, nil, resourceType.Version)
	versions, err := checkResourceType.Check(context.TODO(), processSpec, resourceTypeContainer)
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

	owner := db.NewImageCheckContainerOwner(container, i.teamID)

	imageContainer, err := i.worker.FindOrCreateContainer(
		ctx,
		logger,
		i.imageFetchingDelegate,
		owner,
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		resourceSpec,
		i.customTypes,
	)
	if err != nil {
		return nil, err
	}

	processSpec := runtime.ProcessSpec{
		Path: "/opt/resource/check",
	}
	checkingResource := i.resourceFactory.NewResource(i.imageResource.Source, nil, i.imageResource.Version)
	versions, err := checkingResource.Check(context.TODO(), processSpec, imageContainer)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, ErrImageUnavailable
	}

	return versions[0], nil
}

type fileReadMultiCloser struct {
	reader  io.Reader
	closers []io.Closer
}

func (frc fileReadMultiCloser) Read(p []byte) (n int, err error) {
	return frc.reader.Read(p)
}

func (frc fileReadMultiCloser) Close() error {
	var closeErrors error

	for _, closer := range frc.closers {
		err := closer.Close()
		if err != nil {
			closeErrors = multierror.Append(closeErrors, err)
		}
	}

	return closeErrors
}

func lockName(resourceJson []byte, workerName string) string {
	jsonRes := append(resourceJson, []byte(workerName)...)
	return fmt.Sprintf("%x", sha256.Sum256(jsonRes))
}
