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

const GetResourceLockInterval = 5 * time.Second

var ErrFailedToGetLock = errors.New("failed-to-get-lock")
var ErrInterrupted = errors.New("interrupted")

//go:generate counterfeiter . Fetcher

type Fetcher interface {
	Fetch(
		logger lager.Logger,
		session Session,
		tags atc.Tags,
		teamID int,
		resourceTypes atc.ResourceTypes,
		resourceInstance ResourceInstance,
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
	LockName(workerName string) (string, error)
}

func NewFetcher(
	clock clock.Clock,
	db LockDB,
	fetchSourceProviderFactory FetchSourceProviderFactory,
) Fetcher {
	return &fetcher{
		clock: clock,
		db:    db,
		fetchSourceProviderFactory: fetchSourceProviderFactory,
	}
}

type fetcher struct {
	clock                      clock.Clock
	db                         LockDB
	fetchSourceProviderFactory FetchSourceProviderFactory
}

func (f *fetcher) Fetch(
	logger lager.Logger,
	session Session,
	tags atc.Tags,
	teamID int,
	resourceTypes atc.ResourceTypes,
	resourceInstance ResourceInstance,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	resourceOptions ResourceOptions,
	signals <-chan os.Signal,
	ready chan<- struct{},
) (FetchSource, error) {
	sourceProvider := f.fetchSourceProviderFactory.NewFetchSourceProvider(
		logger,
		session,
		metadata,
		tags,
		teamID,
		resourceTypes,
		resourceInstance,
		resourceOptions,
		imageFetchingDelegate,
	)

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	fetchSource, err := f.fetchWithLock(logger, sourceProvider, resourceOptions.IOConfig(), signals, ready)
	if err != ErrFailedToGetLock {
		return fetchSource, err
	}

	for {
		select {
		case <-ticker.C():
			fetchSource, err := f.fetchWithLock(logger, sourceProvider, resourceOptions.IOConfig(), signals, ready)
			if err != nil {
				if err == ErrFailedToGetLock {
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

func (f *fetcher) fetchWithLock(
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

	lockName, err := source.LockName()
	if err != nil {
		return nil, err
	}

	lockLogger := logger.Session("lock-task", lager.Data{"lock-name": lockName})
	lockLogger.Info("tick")

	lock, acquired, err := f.db.GetTaskLock(lockLogger, lockName)

	if err != nil {
		lockLogger.Error("failed-to-get-lock", err)
		return nil, ErrFailedToGetLock
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return nil, ErrFailedToGetLock
	}

	defer lock.Release()

	err = source.Initialize(signals, ready)
	if err != nil {
		return nil, err
	}

	return source, nil
}
