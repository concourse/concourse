package image

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . LeaseDB

type LeaseDB interface {
	GetLease(logger lager.Logger, leaseName string, interval time.Duration) (db.Lease, bool, error)
}

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(client worker.Client) resource.Tracker
}

type factory struct {
	trackerFactory TrackerFactory
	db             LeaseDB
	clock          clock.Clock
}

func NewFactory(
	trackerFactory TrackerFactory,
	db LeaseDB,
	clock clock.Clock,
) worker.ImageFactory {
	return &factory{
		trackerFactory: trackerFactory,
		db:             db,
		clock:          clock,
	}
}

func (f *factory) NewImage(
	logger lager.Logger,
	signals <-chan os.Signal,
	imageResource atc.ImageResource,
	workerID worker.Identifier,
	workerMetadata worker.Metadata,
	workerTags atc.Tags,
	customTypes atc.ResourceTypes,
	workerClient worker.Client,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	privileged bool,
) worker.Image {
	return &image{
		logger:                logger,
		db:                    f.db,
		signals:               signals,
		imageResource:         imageResource,
		workerID:              workerID,
		workerMetadata:        workerMetadata,
		workerTags:            workerTags,
		customTypes:           customTypes,
		workerClient:          workerClient,
		imageFetchingDelegate: imageFetchingDelegate,
		tracker:               f.trackerFactory.TrackerFor(workerClient),
		clock:                 f.clock,
		privileged:            privileged,
	}
}
