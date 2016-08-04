package image

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type factory struct {
	trackerFactory         resource.TrackerFactory
	resourceFetcherFactory resource.FetcherFactory
}

func NewFactory(
	trackerFactory resource.TrackerFactory,
	resourceFetcherFactory resource.FetcherFactory,
) worker.ImageFactory {
	return &factory{
		trackerFactory:         trackerFactory,
		resourceFetcherFactory: resourceFetcherFactory,
	}
}

func (f *factory) NewImage(
	logger lager.Logger,
	signals <-chan os.Signal,
	imageResource atc.ImageResource,
	workerID worker.Identifier,
	workerMetadata worker.Metadata,
	workerTags atc.Tags,
	teamID int,
	customTypes atc.ResourceTypes,
	workerClient worker.Client,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	privileged bool,
) worker.Image {
	return &image{
		logger:                logger,
		signals:               signals,
		imageResource:         imageResource,
		workerID:              workerID,
		workerMetadata:        workerMetadata,
		workerTags:            workerTags,
		teamID:                teamID,
		customTypes:           customTypes,
		workerClient:          workerClient,
		imageFetchingDelegate: imageFetchingDelegate,
		tracker:               f.trackerFactory.TrackerFor(workerClient),
		resourceFetcher:       f.resourceFetcherFactory.FetcherFor(workerClient),
		privileged:            privileged,
	}
}
