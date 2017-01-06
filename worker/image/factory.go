package image

import (
	"io"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type Factory interface {
	NewImage(
		logger lager.Logger,
		cancel <-chan os.Signal,
		imageResource atc.ImageResource,
		id worker.Identifier,
		metadata worker.Metadata,
		tags atc.Tags,
		teamID int,
		resourceTypes atc.ResourceTypes,
		workerClient worker.Client,
		delegate worker.ImageFetchingDelegate,
		privileged bool,
	) Image
}

type Image interface {
	Fetch() (worker.Volume, io.ReadCloser, atc.Version, error)
}

type factory struct {
	resourceFetcherFactory resource.FetcherFactory
	resourceFactoryFactory resource.ResourceFactoryFactory
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewFactory(
	resourceFetcherFactory resource.FetcherFactory,
	resourceFactoryFactory resource.ResourceFactoryFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) Factory {
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
) Image {
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
