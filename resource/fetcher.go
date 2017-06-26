package resource

import (
	"errors"
	"io"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
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
		resourceTypes creds.VersionedResourceTypes,
		resourceInstance ResourceInstance,
		metadata Metadata,
		imageFetchingDelegate worker.ImageFetchingDelegate,
		signals <-chan os.Signal,
		ready chan<- struct{},
	) (VersionedSource, error)
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
	resourceTypes creds.VersionedResourceTypes,
	resourceInstance ResourceInstance,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
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
		imageFetchingDelegate,
	)

	source, err := sourceProvider.Get()
	if err != nil {
		return nil, err
	}

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	versionedSource, err := f.fetchWithLock(logger, source, imageFetchingDelegate.Stdout(), signals, ready)
	if err != ErrFailedToGetLock {
		return versionedSource, err
	}

	for {
		select {
		case <-ticker.C():
			versionedSource, err := f.fetchWithLock(logger, source, imageFetchingDelegate.Stdout(), signals, ready)
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
	stdout io.Writer,
	signals <-chan os.Signal,
	ready chan<- struct{},
) (VersionedSource, error) {
	versionedSource, found, err := source.Find()
	if err != nil {
		return nil, err
	}

	if found {
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
