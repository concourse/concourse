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
	imageConfig atc.TaskImageConfig,
	signals <-chan os.Signal,
	identifier worker.Identifier,
	metadata worker.Metadata,
	delegate worker.ImageFetchingDelegate,
	worker worker.Client,
	workerTags atc.Tags,
	customTypes atc.ResourceTypes,
) (worker.Image, error) {
	tracker := fetcher.trackerFactory.TrackerFor(worker)
	resourceType := resource.ResourceType(imageConfig.Type)

	checkSess := resource.Session{
		ID:       identifier,
		Metadata: metadata,
	}

	checkSess.ID.Stage = db.ContainerStageCheck
	checkSess.ID.ImageResourceType = imageConfig.Type
	checkSess.ID.ImageResourceSource = imageConfig.Source
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

	versions, err := checkingResource.Check(imageConfig.Source, nil)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, ErrImageUnavailable
	}

	cacheID := resource.ResourceCacheIdentifier{
		Type:    resourceType,
		Version: versions[0],
		Source:  imageConfig.Source,
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
	getSess.ID.ImageResourceType = imageConfig.Type
	getSess.ID.ImageResourceSource = imageConfig.Source
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
		imageConfig.Source,
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
	}, nil
}

type resourceImage struct {
	volume   worker.Volume
	metadata worker.ImageMetadata
	resource resource.Resource
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

func loadMetadata(source resource.VersionedSource) (worker.ImageMetadata, error) {
	reader, err := source.StreamOut(imageMetadataFile)
	if err != nil {
		return worker.ImageMetadata{}, err
	}

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
