package resource

import (
	"context"
	"errors"
	"io"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/worker"
)

const GetResourceLockInterval = 5 * time.Second

var ErrFailedToGetLock = errors.New("failed-to-get-lock")
var ErrInterrupted = errors.New("interrupted")

//go:generate counterfeiter . Fetcher

type Fetcher interface {
	Fetch(
		ctx context.Context,
		logger lager.Logger,
		session Session,
		tags atc.Tags,
		teamID int,
		resourceTypes creds.VersionedResourceTypes,
		resourceInstance ResourceInstance,
		metadata Metadata,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (worker.Volume, error)
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
	ctx context.Context,
	logger lager.Logger,
	session Session,
	tags atc.Tags,
	teamID int,
	resourceTypes creds.VersionedResourceTypes,
	resourceInstance ResourceInstance,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (worker.Volume, error) {
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

	volume, err := f.fetchWithLock(ctx, logger, source, imageFetchingDelegate.Stdout())
	if err != ErrFailedToGetLock {
		return volume, err
	}

	for {
		select {
		case <-ticker.C():
			volume, err := f.fetchWithLock(ctx, logger, source, imageFetchingDelegate.Stdout())
			if err != nil {
				if err == ErrFailedToGetLock {
					break
				}
				return nil, err
			}

			return volume, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (f *fetcher) fetchWithLock(
	ctx context.Context,
	logger lager.Logger,
	source FetchSource,
	stdout io.Writer,
) (worker.Volume, error) {
	volume, found, err := source.Find()
	if err != nil {
		return nil, err
	}

	if found {
		return volume, nil
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

	return source.Create(ctx)
}
