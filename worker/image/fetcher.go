package image

import (
	"errors"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

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
	imageConfig atc.TaskImageConfig,
	signals <-chan os.Signal,
	identifier worker.Identifier,
	metadata worker.Metadata,
	delegate worker.ImageFetchingDelegate,
	worker worker.Client,
) (worker.Image, error) {
	tracker := fetcher.trackerFactory.TrackerFor(worker)
	resourceType := resource.ResourceType(imageConfig.Type)

	checkSess := resource.Session{
		ID:       identifier,
		Metadata: metadata,
	}

	checkSess.ID.Stage = db.ContainerStageCheck
	checkSess.ID.CheckType = imageConfig.Type
	checkSess.ID.CheckSource = imageConfig.Source
	checkSess.Metadata.Type = db.ContainerTypeCheck
	checkSess.Metadata.WorkingDirectory = ""
	checkSess.Metadata.EnvironmentVariables = nil

	checkingResource, err := tracker.Init(
		logger.Session("check-image"),
		resource.EmptyMetadata{},
		checkSess,
		resourceType,
		nil,
	)
	if err != nil {
		return nil, err
	}

	defer checkingResource.Release(0)

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
	getSess.Metadata.Type = db.ContainerTypeGet
	getSess.Metadata.WorkingDirectory = ""
	getSess.Metadata.EnvironmentVariables = nil

	getResource, cache, err := tracker.InitWithCache(
		logger.Session("init-image"),
		resource.EmptyMetadata{},
		getSess,
		resourceType,
		nil,
		cacheID,
	)
	if err != nil {
		return nil, err
	}

	isInitialized, err := cache.IsInitialized()
	if err != nil {
		return nil, err
	}

	if !isInitialized {
		versionedSource := getResource.Get(
			resource.IOConfig{
				Stderr: delegate.Stderr(),
			},
			imageConfig.Source,
			nil,
			versions[0],
		)

		err := versionedSource.Run(signals, make(chan struct{}))
		if err != nil {
			return nil, err
		}

		cache.Initialize()
	}

	volume, found, err := getResource.CacheVolume()
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrImageGetDidNotProduceVolume
	}

	return resourceImage{
		volume: volume,
	}, nil
}

type resourceImage struct {
	volume worker.Volume
}

func (image resourceImage) Volume() worker.Volume {
	return image.volume
}
