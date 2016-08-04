package resource

import (
	"errors"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

const GetResourceLeaseInterval = 5 * time.Second

var ErrFailedToGetLease = errors.New("failed-to-get-lease")
var ErrInterrupted = errors.New("interrupted")

//go:generate counterfeiter . Fetcher

type Fetcher interface {
	Fetch(
		logger lager.Logger,
		session Session,
		tags atc.Tags,
		teamID int,
		resourceTypes atc.ResourceTypes,
		cacheIdentifier CacheIdentifier,
		metadata Metadata,
		imageFetchingDelegate worker.ImageFetchingDelegate,
		resourceOptions ResourceOptions,
		signals <-chan os.Signal,
		ready chan<- struct{},
	) (FetchSource, error)
}

//go:generate counterfeiter . ResourceOptions

type ResourceOptions interface {
	IOConfig() IOConfig
	Source() atc.Source
	Params() atc.Params
	Version() atc.Version
	ResourceType() ResourceType
	LeaseName(workerName string) (string, error)
}

func NewFetcher(
	clock clock.Clock,
	db LeaseDB,
	fetchContainerCreatorFactory FetchContainerCreatorFactory,
	fetchSourceProviderFactory FetchSourceProviderFactory,
) Fetcher {
	return &fetcher{
		clock: clock,
		db:    db,
		fetchContainerCreatorFactory: fetchContainerCreatorFactory,
		fetchSourceProviderFactory:   fetchSourceProviderFactory,
	}
}

type fetcher struct {
	clock                        clock.Clock
	db                           LeaseDB
	fetchContainerCreatorFactory FetchContainerCreatorFactory
	fetchSourceProviderFactory   FetchSourceProviderFactory
}

func (f *fetcher) Fetch(
	logger lager.Logger,
	session Session,
	tags atc.Tags,
	teamID int,
	resourceTypes atc.ResourceTypes,
	cacheIdentifier CacheIdentifier,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	resourceOptions ResourceOptions,
	signals <-chan os.Signal,
	ready chan<- struct{},
) (FetchSource, error) {
	containerCreator := f.fetchContainerCreatorFactory.NewFetchContainerCreator(
		logger,
		resourceTypes,
		tags,
		teamID,
		session,
		metadata,
		imageFetchingDelegate,
	)

	sourceProvider := f.fetchSourceProviderFactory.NewFetchSourceProvider(
		logger,
		session,
		tags,
		teamID,
		resourceTypes,
		cacheIdentifier,
		resourceOptions,
		containerCreator,
	)

	ticker := f.clock.NewTicker(GetResourceLeaseInterval)
	defer ticker.Stop()

	fetchSource, err := f.fetchWithLease(logger, sourceProvider, resourceOptions.IOConfig(), signals, ready)
	if err != ErrFailedToGetLease {
		return fetchSource, err
	}

	for {
		select {
		case <-ticker.C():
			fetchSource, err := f.fetchWithLease(logger, sourceProvider, resourceOptions.IOConfig(), signals, ready)
			if err != nil {
				if err == ErrFailedToGetLease {
					break
				}
				return nil, err
			}

			return fetchSource, nil

		case <-signals:
			return nil, ErrInterrupted
		}
	}
}

func (f *fetcher) fetchWithLease(
	logger lager.Logger,
	sourceProvider FetchSourceProvider,
	ioConfig IOConfig,
	signals <-chan os.Signal,
	ready chan<- struct{},
) (FetchSource, error) {
	source, err := sourceProvider.Get()
	if err != nil {
		return nil, err
	}

	isInitialized, err := source.IsInitialized()
	if err != nil {
		return nil, err
	}

	if isInitialized {
		if ioConfig.Stdout != nil {
			fmt.Fprintf(ioConfig.Stdout, "using version of resource found in cache\n")
		}
		close(ready)
		return source, nil
	}

	leaseName, err := source.LeaseName()
	if err != nil {
		return nil, err
	}

	leaseLogger := logger.Session("lease-task", lager.Data{"lease-name": leaseName})
	leaseLogger.Info("tick")

	lease, leased, err := f.db.GetLease(leaseLogger, leaseName, GetResourceLeaseInterval)

	if err != nil {
		leaseLogger.Error("failed-to-get-lease", err)
		return nil, ErrFailedToGetLease
	}

	if !leased {
		leaseLogger.Debug("did-not-get-lease")
		return nil, ErrFailedToGetLease
	}

	defer lease.Break()

	err = source.Initialize(signals, ready)
	if err != nil {
		return nil, err
	}

	return source, nil
}
