package resource

import (
	"errors"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
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
		resourceTypes atc.VersionedResourceTypes,
		resourceInstance ResourceInstance,
		metadata Metadata,
		imageFetchingDelegate worker.ImageFetchingDelegate,
		resourceOptions ResourceOptions,
		signals <-chan os.Signal,
		ready chan<- struct{},
	) (VersionedSource, error)
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
	lockFactory lock.LockFactory,
	fetchSourceProviderFactory FetchSourceProviderFactory,
) Fetcher {
	return &fetcher{
		clock:                      clock,
		lockFactory:                lockFactory,
		fetchSourceProviderFactory: fetchSourceProviderFactory,
	}
}

type fetcher struct {
	clock                      clock.Clock
	lockFactory                lock.LockFactory
	fetchSourceProviderFactory FetchSourceProviderFactory
}

func (f *fetcher) Fetch(
	logger lager.Logger,
	session Session,
	tags atc.Tags,
	teamID int,
	resourceTypes atc.VersionedResourceTypes,
	resourceInstance ResourceInstance,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	resourceOptions ResourceOptions,
	signals <-chan os.Signal,
	ready chan<- struct{},
) (VersionedSource, error) {
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

	source, err := sourceProvider.Get()
	if err != nil {
		return nil, err
	}

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	versionedSource, err := f.fetchWithLock(logger, source, resourceOptions.IOConfig(), signals, ready)
	if err != ErrFailedToGetLock {
		return versionedSource, err
	}

	for {
		select {
		case <-ticker.C():
			versionedSource, err := f.fetchWithLock(logger, source, resourceOptions.IOConfig(), signals, ready)
			if err != nil {
				if err == ErrFailedToGetLock {
					break
				}
				return nil, err
			}

			return versionedSource, nil

		case <-signals:
			return nil, ErrInterrupted
		}
	}
}

func (f *fetcher) fetchWithLock(
	logger lager.Logger,
	source FetchSource,
	ioConfig IOConfig,
	signals <-chan os.Signal,
	ready chan<- struct{},
) (VersionedSource, error) {
	versionedSource, found, err := source.Find()
	if err != nil {
		return nil, err
	}

	if found {
		if ioConfig.Stdout != nil {
			fmt.Fprintf(ioConfig.Stdout, "using version of resource found in cache\n")
		}
		close(ready)
		return versionedSource, nil
	}

	lockName, err := source.LockName()
	if err != nil {
		return nil, err
	}

	lockLogger := logger.Session("lock-task", lager.Data{"lock-name": lockName})

	lock, acquired, err := f.lockFactory.Acquire(lockLogger, lock.NewTaskLockID(lockName))
	if err != nil {
		lockLogger.Error("failed-to-get-lock", err)
		return nil, ErrFailedToGetLock
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return nil, ErrFailedToGetLock
	}

	defer lock.Release()

	return source.Create(signals, ready)
}
