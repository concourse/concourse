package image

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

const imageMetadataFile = "metadata.json"

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(client worker.Client) resource.Tracker
}

// ErrImageUnavailable is returned when a task's configured image resource
// has no versions.
var ErrImageUnavailable = errors.New("no versions of image available")

var ErrImageGetDidNotProduceVolume = errors.New("fetching the image did not produce a volume")

type MalformedMetadataError struct {
	UnmarshalError error
}

func (err MalformedMetadataError) Error() string {
	return fmt.Sprintf("malformed image metadata: %s", err.UnmarshalError)
}

type Fetcher struct {
	trackerFactory TrackerFactory
}

func NewFetcher(trackerFactory TrackerFactory) Fetcher {
	return Fetcher{
		trackerFactory: trackerFactory,
	}
}

func (fetcher Fetcher) FetchImage(
	logger lager.Logger,
	imageResource atc.ImageResource,
	signals <-chan os.Signal,
	identifier worker.Identifier,
	metadata worker.Metadata,
	delegate worker.ImageFetchingDelegate,
	worker worker.Client,
	workerTags atc.Tags,
	customTypes atc.ResourceTypes,
) (worker.Image, error) {
	tracker := fetcher.trackerFactory.TrackerFor(worker)
	resourceType := resource.ResourceType(imageResource.Type)

	checkSess := resource.Session{
		ID:       identifier,
		Metadata: metadata,
	}

	checkSess.ID.Stage = db.ContainerStageCheck
	checkSess.ID.ImageResourceType = imageResource.Type
	checkSess.ID.ImageResourceSource = imageResource.Source
	checkSess.Metadata.Type = db.ContainerTypeCheck
	checkSess.Metadata.WorkingDirectory = ""
	checkSess.Metadata.EnvironmentVariables = nil

	checkingResource, err := tracker.Init(
		logger.Session("check-image"),
		resource.EmptyMetadata{},
		checkSess,
		resourceType,
		workerTags,
		customTypes,
		delegate,
	)
	if err != nil {
		return nil, err
	}

	defer checkingResource.Release(nil)

	versions, err := checkingResource.Check(imageResource.Source, nil)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, ErrImageUnavailable
	}

	cacheID := resource.ResourceCacheIdentifier{
		Type:    resourceType,
		Version: versions[0],
		Source:  imageResource.Source,
	}

	volumeID := cacheID.VolumeIdentifier()

	err = delegate.ImageVersionDetermined(volumeID)
	if err != nil {
		return nil, err
	}

	getSess := resource.Session{
		ID:       identifier,
		Metadata: metadata,
	}

	getSess.ID.Stage = db.ContainerStageGet
	getSess.ID.ImageResourceType = imageResource.Type
	getSess.ID.ImageResourceSource = imageResource.Source
	getSess.Metadata.Type = db.ContainerTypeGet
	getSess.Metadata.WorkingDirectory = ""
	getSess.Metadata.EnvironmentVariables = nil

	getResource, cache, err := tracker.InitWithCache(
		logger.Session("init-image"),
		resource.EmptyMetadata{},
		getSess,
		resourceType,
		workerTags,
		cacheID,
		customTypes,
		delegate,
	)
	if err != nil {
		return nil, err
	}

	isInitialized, err := cache.IsInitialized()
	if err != nil {
		return nil, err
	}

	versionedSource := getResource.Get(
		resource.IOConfig{
			Stderr: delegate.Stderr(),
		},
		imageResource.Source,
		nil,
		versions[0],
	)

	if !isInitialized {
		err := versionedSource.Run(signals, make(chan struct{}))
		if err != nil {
			return nil, err
		}

		err = cache.Initialize()
		if err != nil {
			return nil, err
		}
	}

	volume, found := getResource.CacheVolume()
	if !found {
		return nil, ErrImageGetDidNotProduceVolume
	}

	imageMetadata, err := loadMetadata(versionedSource)
	if err != nil {
		return nil, err
	}

	return resourceImage{
		volume:   volume,
		metadata: imageMetadata,
		resource: getResource,
		version:  versions[0],
	}, nil
}

type resourceImage struct {
	volume   worker.Volume
	metadata worker.ImageMetadata
	resource resource.Resource
	version  atc.Version
}

func (image resourceImage) Volume() worker.Volume {
	return image.volume
}

func (image resourceImage) Metadata() worker.ImageMetadata {
	return image.metadata
}

func (image resourceImage) Release(finalTTL *time.Duration) {
	image.resource.Release(finalTTL)
}

func (image resourceImage) Version() atc.Version {
	return image.version
}

func loadMetadata(source resource.VersionedSource) (worker.ImageMetadata, error) {
	reader, err := source.StreamOut(imageMetadataFile)
	if err != nil {
		return worker.ImageMetadata{}, err
	}

	defer reader.Close()

	tarReader := tar.NewReader(reader)

	_, err = tarReader.Next()
	if err != nil {
		return worker.ImageMetadata{}, errors.New("could not read file from tar")
	}

	var imageMetadata worker.ImageMetadata
	if err = json.NewDecoder(tarReader).Decode(&imageMetadata); err != nil {
		return worker.ImageMetadata{}, MalformedMetadataError{
			UnmarshalError: err,
		}
	}

	return imageMetadata, nil
}
