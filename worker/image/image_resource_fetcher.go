package image

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
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
	ImageResourceFetcherFor(worker.Worker) ImageResourceFetcher
}

//go:generate counterfeiter . ImageResourceFetcher

type ImageResourceFetcher interface {
	Fetch(
		logger lager.Logger,
		signals <-chan os.Signal,
		resourceUser dbng.ResourceUser,
		imageResourceType string,
		imageResourceSource atc.Source,
		tags atc.Tags,
		teamID int,
		customTypes atc.VersionedResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
		privileged bool,
	) (worker.Volume, io.ReadCloser, atc.Version, error)
}

type imageResourceFetcherFactory struct {
	resourceFetcherFactory  resource.FetcherFactory
	resourceFactoryFactory  resource.ResourceFactoryFactory
	dbResourceCacheFactory  dbng.ResourceCacheFactory
	dbResourceConfigFactory dbng.ResourceConfigFactory
	clock                   clock.Clock
}

func NewImageResourceFetcherFactory(
	resourceFetcherFactory resource.FetcherFactory,
	resourceFactoryFactory resource.ResourceFactoryFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	clock clock.Clock,
) ImageResourceFetcherFactory {
	return &imageResourceFetcherFactory{
		resourceFetcherFactory:  resourceFetcherFactory,
		resourceFactoryFactory:  resourceFactoryFactory,
		dbResourceCacheFactory:  dbResourceCacheFactory,
		dbResourceConfigFactory: dbResourceConfigFactory,
		clock: clock,
	}
}

func (f *imageResourceFetcherFactory) ImageResourceFetcherFor(worker worker.Worker) ImageResourceFetcher {
	return &imageResourceFetcher{
		resourceFetcher:         f.resourceFetcherFactory.FetcherFor(worker),
		resourceFactory:         f.resourceFactoryFactory.FactoryFor(worker),
		dbResourceCacheFactory:  f.dbResourceCacheFactory,
		dbResourceConfigFactory: f.dbResourceConfigFactory,
		clock: f.clock,
	}
}

type imageResourceFetcher struct {
	resourceFetcher         resource.Fetcher
	resourceFactory         resource.ResourceFactory
	dbResourceCacheFactory  dbng.ResourceCacheFactory
	dbResourceConfigFactory dbng.ResourceConfigFactory
	clock                   clock.Clock
}

func (i *imageResourceFetcher) Fetch(
	logger lager.Logger,
	signals <-chan os.Signal,
	resourceUser dbng.ResourceUser,
	imageResourceType string,
	imageResourceSource atc.Source,
	tags atc.Tags,
	teamID int,
	customTypes atc.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	privileged bool,
) (worker.Volume, io.ReadCloser, atc.Version, error) {
	version, err := i.getLatestVersion(logger, signals, resourceUser, imageResourceType, imageResourceSource, tags, teamID, customTypes, imageFetchingDelegate)
	if err != nil {
		logger.Error("failed-to-get-latest-image-version", err)
		return nil, nil, nil, err
	}

	resourceInstance := resource.NewResourceInstance(
		resource.ResourceType(imageResourceType),
		version,
		imageResourceSource,
		atc.Params{},
		resourceUser,
		customTypes,
		i.dbResourceCacheFactory,
	)

	err = imageFetchingDelegate.ImageVersionDetermined(
		resourceInstance.ResourceCacheIdentifier(),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	getSess := resource.Session{
		Metadata: dbng.ContainerMetadata{
			Type: dbng.ContainerTypeGet,
		},
	}

	resourceType := resource.ResourceType(imageResourceType)

	resourceOptions := &imageResourceOptions{
		imageFetchingDelegate: imageFetchingDelegate,
		source:                imageResourceSource,
		version:               version,
		resourceType:          resourceType,
	}

	// we need resource cache for build
	versionedSource, err := i.resourceFetcher.Fetch(
		logger.Session("init-image"),
		getSess,
		tags,
		teamID,
		customTypes,
		resourceInstance,
		resource.EmptyMetadata{},
		imageFetchingDelegate,
		resourceOptions,
		signals,
		make(chan struct{}),
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

	tarReader := tar.NewReader(reader)

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

func (i *imageResourceFetcher) getLatestVersion(
	logger lager.Logger,
	signals <-chan os.Signal,
	resourceUser dbng.ResourceUser,
	imageResourceType string,
	imageResourceSource atc.Source,
	tags atc.Tags,
	teamID int,
	customTypes atc.VersionedResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (atc.Version, error) {
	resourceSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: imageResourceType,
			Privileged:   true,
		},
		Tags:   tags,
		TeamID: teamID,
	}

	for {
		lock, acquired, err := i.dbResourceConfigFactory.AcquireResourceCheckingLock(
			logger,
			resourceUser,
			imageResourceType,
			imageResourceSource,
			customTypes,
		)
		if err != nil {
			logger.Error("failed-to-get-lock", err, lager.Data{
				"resource-user": resourceUser,
			})

			return nil, err
		}

		if !acquired {
			logger.Debug("did-not-get-lock")
			i.clock.Sleep(time.Second)
			continue
		}

		defer lock.Release()

		break
	}

	checkingResource, err := i.resourceFactory.NewCheckResource(
		logger,
		signals,
		resourceUser,
		imageResourceType,
		imageResourceSource,
		dbng.ContainerMetadata{
			Type: dbng.ContainerTypeCheck,
		},
		resourceSpec,
		customTypes,
		imageFetchingDelegate,
	)
	if err != nil {
		return nil, err
	}

	versions, err := checkingResource.Check(imageResourceSource, nil)
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

type imageResourceOptions struct {
	imageFetchingDelegate worker.ImageFetchingDelegate
	source                atc.Source
	version               atc.Version
	resourceType          resource.ResourceType
}

func (d *imageResourceOptions) IOConfig() resource.IOConfig {
	return resource.IOConfig{
		Stderr: d.imageFetchingDelegate.Stderr(),
	}
}

func (ir *imageResourceOptions) Source() atc.Source {
	return ir.source
}

func (ir *imageResourceOptions) Params() atc.Params {
	return nil
}

func (ir *imageResourceOptions) Version() atc.Version {
	return ir.version
}

func (ir *imageResourceOptions) ResourceType() resource.ResourceType {
	return ir.resourceType
}

func (ir *imageResourceOptions) LockName(workerName string) (string, error) {
	id := &leaseID{
		Type:       ir.resourceType,
		Version:    ir.version,
		Source:     ir.source,
		WorkerName: workerName,
	}

	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
}

type readCloser struct {
	io.Reader
	io.Closer
}
