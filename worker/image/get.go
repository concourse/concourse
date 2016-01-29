package image

import (
	"errors"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . TaskDelegate

type TaskDelegate interface {
	SaveImageResourceVersion(db.VolumeIdentifier) error

	Stdout() io.Writer
	Stderr() io.Writer
}

var ErrImageUnavailable = errors.New("no versions of image available")

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(client worker.Client) resource.Tracker
}

func GetContainerImage(logger lager.Logger, signals <-chan os.Signal, trackerFactory TrackerFactory, identifier worker.Identifier, metadata worker.Metadata, delegate TaskDelegate, worker worker.Client, config atc.TaskConfig) (resource.Resource, error) {
	tracker := trackerFactory.TrackerFor(worker)
	resourceType := resource.ResourceType(config.ImageResource.Type)

	checkSess := resource.Session{
		ID:       identifier,
		Metadata: metadata,
	}

	checkSess.ID.Stage = db.ContainerStageCheck
	checkSess.ID.CheckType = config.ImageResource.Type
	checkSess.ID.CheckSource = config.ImageResource.Source
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

	versions, err := checkingResource.Check(config.ImageResource.Source, nil)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, ErrImageUnavailable
	}

	cacheID := resource.ResourceCacheIdentifier{
		Type:    resourceType,
		Version: versions[0],
		Source:  config.ImageResource.Source,
	}

	volumeID := cacheID.VolumeIdentifier()

	err = delegate.SaveImageResourceVersion(volumeID)
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
			config.ImageResource.Source,
			nil,
			versions[0],
		)

		err := versionedSource.Run(signals, make(chan struct{}))
		if err != nil {
			return nil, err
		}

		cache.Initialize()
	}

	return getResource, nil
}
