package worker

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc/resource"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
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
		gardenWorker Worker,
		containerSpec ContainerSpec,
		processSpec runtime.ProcessSpec,
		resource resource.Resource,
		owner db.ContainerOwner,
		imageFetcherSpec ImageFetcherSpec,
		cache db.UsedResourceCache,
		lockName string,
	) (GetResult, Volume, error)
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
	gardenWorker Worker,
	containerSpec ContainerSpec,
	processSpec runtime.ProcessSpec,
	resource resource.Resource,
	owner db.ContainerOwner,
	imageFetcherSpec ImageFetcherSpec,
	cache db.UsedResourceCache,
	lockName string,
) (GetResult, Volume, error) {
	result := GetResult{}
	var volume Volume
	containerSpec.Outputs = map[string]string{
		"resource": processSpec.Dir,
	}

	fetchSource := f.fetchSourceFactory.NewFetchSource(logger, gardenWorker, owner,
		cache, resource, imageFetcherSpec.ResourceTypes, containerSpec, processSpec,
		containerMetadata, imageFetcherSpec.Delegate)

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	result, volume, err := f.fetchWithLock(ctx, logger, fetchSource, imageFetcherSpec.Delegate.Stdout(), cache, lockName)
	if err == nil || err != ErrFailedToGetLock {
		return result, volume, err
	}

	for {
		select {
		case <-ticker.C():
			//TODO this is called redundantly?
			result, volume, err = f.fetchWithLock(ctx, logger, fetchSource, imageFetcherSpec.Delegate.Stdout(), cache, lockName)
			if err != nil {
				if err == ErrFailedToGetLock {
					break
				}
				return result, nil, err
			}

			return result, volume, nil

		case <-ctx.Done():
			return GetResult{}, nil, ctx.Err()
		}
	}
}

func (f *fetcher) fetchWithLock(
	ctx context.Context,
	logger lager.Logger,
	source FetchSource,
	stdout io.Writer,
	cache db.UsedResourceCache,
	lockName string,
) (GetResult, Volume, error) {
	result := GetResult{}
	findResult, volume, found, err := source.Find()
	if err != nil {
		return result, nil, err
	}

	if found {
		return findResult, volume, nil
	}

	lockLogger := logger.Session("lock-task", lager.Data{"lock-name": lockName})

	lock, acquired, err := f.lockFactory.Acquire(lockLogger, lock.NewTaskLockID(lockName))
	if err != nil {
		lockLogger.Error("failed-to-get-lock", err)
		return result, nil, ErrFailedToGetLock
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return result, nil, ErrFailedToGetLock
	}

	defer lock.Release()

	return source.Create(ctx)
}
