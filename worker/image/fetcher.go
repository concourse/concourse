package image

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

const ImageMetadataFile = "metadata.json"

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(client worker.Client) resource.Tracker
}

// ErrImageUnavailable is returned when a task's configured image resource
// has no versions.
var ErrImageUnavailable = errors.New("no versions of image available")

var ErrImageGetDidNotProduceVolume = errors.New("fetching the image did not produce a volume")

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
	workerClient worker.Client,
	workerTags atc.Tags,
	customTypes atc.ResourceTypes,
	privileged bool,
) (worker.Volume, io.ReadCloser, atc.Version, error) {
	tracker := fetcher.trackerFactory.TrackerFor(workerClient)
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
		return nil, nil, nil, err
	}

	defer checkingResource.Release(nil)

	versions, err := checkingResource.Check(imageResource.Source, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	if len(versions) == 0 {
		return nil, nil, nil, ErrImageUnavailable
	}

	cacheID := resource.ResourceCacheIdentifier{
		Type:    resourceType,
		Version: versions[0],
		Source:  imageResource.Source,
	}

	volumeID := cacheID.VolumeIdentifier()

	err = delegate.ImageVersionDetermined(volumeID)
	if err != nil {
		return nil, nil, nil, err
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
		return nil, nil, nil, err
	}

	isInitialized, err := cache.IsInitialized()
	if err != nil {
		return nil, nil, nil, err
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
			return nil, nil, nil, err
		}

		err = cache.Initialize()
		if err != nil {
			return nil, nil, nil, err
		}
	}

	volume, found := getResource.CacheVolume()
	if !found {
		return nil, nil, nil, ErrImageGetDidNotProduceVolume
	}

	volumeSpec := worker.VolumeSpec{
		Strategy: worker.ContainerRootFSStrategy{
			Parent: volume,
		},
		Privileged: privileged,
		TTL:        worker.ContainerTTL,
	}
	cowVolume, err := workerClient.CreateVolume(logger.Session("create-cow-volume"), volumeSpec)
	if err != nil {
		return nil, nil, nil, err
	}

	volume.Release(nil)

	reader, err := versionedSource.StreamOut(ImageMetadataFile)
	if err != nil {
		return nil, nil, nil, err
	}

	tarReader := tar.NewReader(reader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read file \"%s\" from tar", ImageMetadataFile)
	}

	releasingReader := &releasingReadCloser{
		Reader:      tarReader,
		Closer:      reader,
		releaseFunc: func() { getResource.Release(nil) },
	}

	return cowVolume, releasingReader, versions[0], nil
}

type releasingReadCloser struct {
	io.Reader
	io.Closer
	releaseFunc func()
}

func (r *releasingReadCloser) Close() error {
	r.releaseFunc()
	return r.Closer.Close()
}
