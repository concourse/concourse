package image

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type factory struct {
	resourceFetcherFactory resource.FetcherFactory
	resourceFactoryFactory resource.ResourceFactoryFactory
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewFactory(
	resourceFetcherFactory resource.FetcherFactory,
	resourceFactoryFactory resource.ResourceFactoryFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) worker.ImageFactory {
	return &factory{
		resourceFetcherFactory: resourceFetcherFactory,
		resourceFactoryFactory: resourceFactoryFactory,
		dbResourceCacheFactory: dbResourceCacheFactory,
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
		logger:                 logger,
		signals:                signals,
		imageResource:          imageResource,
		workerID:               workerID,
		workerMetadata:         workerMetadata,
		workerTags:             workerTags,
		teamID:                 teamID,
		customTypes:            customTypes,
		imageFetchingDelegate:  imageFetchingDelegate,
		resourceFactory:        f.resourceFactoryFactory.FactoryFor(workerClient),
		resourceFetcher:        f.resourceFetcherFactory.FetcherFor(workerClient),
		dbResourceCacheFactory: f.dbResourceCacheFactory,
		privileged:             privileged,
	}
}
