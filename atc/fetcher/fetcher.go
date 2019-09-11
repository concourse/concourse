package fetcher

import (
	"context"
	"errors"
	"io"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

const GetResourceLockInterval = 5 * time.Second

var ErrFailedToGetLock = errors.New("failed to get lock")
var ErrInterrupted = errors.New("interrupted")

//go:generate counterfeiter . Fetcher

type Fetcher interface {
	Fetch(
		ctx context.Context,
		logger lager.Logger,
		containerMetadata db.ContainerMetadata,
		gardenWorker worker.Worker,
		containerSpec worker.ContainerSpec,
		resourceTypes atc.VersionedResourceTypes,
		resourceInstance resource.ResourceInstance,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (resource.VersionedSource, error)
}

func NewFetcher(
	clock clock.Clock,
	lockFactory lock.LockFactory,
	fetchSourceFactory FetchSourceFactory,
) Fetcher {
	return &fetcher{
		clock:              clock,
		lockFactory:        lockFactory,
		fetchSourceFactory: fetchSourceFactory,
	}
}

type fetcher struct {
	clock              clock.Clock
	lockFactory        lock.LockFactory
	fetchSourceFactory FetchSourceFactory
}

func (f *fetcher) Fetch(
	ctx context.Context,
	logger lager.Logger,
	containerMetadata db.ContainerMetadata,
	gardenWorker worker.Worker,
	containerSpec worker.ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
	resourceInstance resource.ResourceInstance,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (resource.VersionedSource, error) {
	containerSpec.Outputs = map[string]string{
		"resource": resource.ResourcesDir("get"),
	}

	source := f.fetchSourceFactory.NewFetchSource(logger, gardenWorker, resourceInstance, resourceTypes, containerSpec, containerMetadata, imageFetchingDelegate)

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	versionedSource, err := f.fetchWithLock(ctx, logger, source, imageFetchingDelegate.Stdout())
	if err != ErrFailedToGetLock {
		return versionedSource, err
	}

	for {
		select {
		case <-ticker.C():
			versionedSource, err := f.fetchWithLock(ctx, logger, source, imageFetchingDelegate.Stdout())
			if err != nil {
				if err == ErrFailedToGetLock {
					break
				}
				return nil, err
			}

			return versionedSource, nil

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
) (resource.VersionedSource, error) {
	versionedSource, found, err := source.Find()
	if err != nil {
		return nil, err
	}

	if found {
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

	return source.Create(ctx)
}
