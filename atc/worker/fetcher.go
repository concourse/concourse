package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
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
		processSpec ProcessSpec,
		resourceTypes atc.VersionedResourceTypes,
		source atc.Source,
		params atc.Params,
		owner db.ContainerOwner,
		resourceDir string,
		resourceInstanceSignature string,
		imageFetchingDelegate ImageFetchingDelegate,
		cache db.UsedResourceCache,
	) (GetResult, error)
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

// TODO: end of the above TODO

func (f *fetcher) Fetch(
	ctx context.Context,
	logger lager.Logger,
	containerMetadata db.ContainerMetadata,
	gardenWorker Worker,
	containerSpec ContainerSpec,
	processSpec ProcessSpec,
	resourceTypes atc.VersionedResourceTypes,
	source atc.Source,
	params atc.Params,
	owner db.ContainerOwner,
	resourceDir string,
	resourceInstanceSignature string,
	imageFetchingDelegate ImageFetchingDelegate,
	cache db.UsedResourceCache,
) (GetResult, error) {
	containerSpec.Outputs = map[string]string{
		"resource": resourceDir,
	}

	fetchSource := f.fetchSourceFactory.NewFetchSource(logger, gardenWorker, source, params, owner, resourceDir, cache, resourceTypes, containerSpec, processSpec, containerMetadata, imageFetchingDelegate)

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	// figure out the lockname earlier, because we have all the info
	lockName, err := lockName(resourceInstanceSignature, gardenWorker.Name())
	if err != nil {
		return GetResult{}, err
	}

	versionedSource, err := f.fetchWithLock(ctx, logger, fetchSource, imageFetchingDelegate.Stdout(), cache, lockName)
	if err != ErrFailedToGetLock {
		return versionedSource, err
	}

	for {
		select {
		case <-ticker.C():
			//TODO this is called redundantly?
			result, err := f.fetchWithLock(ctx, logger, fetchSource, imageFetchingDelegate.Stdout(), cache, lockName)
			if err != nil {
				if err == ErrFailedToGetLock {
					break
				}
				return GetResult{}, err
			}

			return result, nil

		case <-ctx.Done():
			return GetResult{}, ctx.Err()
		}
	}
}

type lockID struct {
	ResourceInstanceSignature string `json:"resource_instance_signature,omitempty"`
	WorkerName string      `json:"worker_name,omitempty"`
}

func lockName(resourceInstanceSignature string, workerName string) (string, error) {
	id := &lockID{
		ResourceInstanceSignature: resourceInstanceSignature,
		WorkerName: workerName,
	}

	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
}

func (f *fetcher) fetchWithLock(
	ctx context.Context,
	logger lager.Logger,
	source FetchSource,
	stdout io.Writer,
	cache db.UsedResourceCache,
	lockName string,
) (GetResult, error) {
	findResult, found, err := source.Find()
	if err != nil {
		return GetResult{}, err
	}

	if found {
		return findResult, nil
	}

	lockLogger := logger.Session("lock-task", lager.Data{"lock-name": lockName})

	lock, acquired, err := f.lockFactory.Acquire(lockLogger, lock.NewTaskLockID(lockName))
	if err != nil {
		lockLogger.Error("failed-to-get-lock", err)
		return GetResult{}, ErrFailedToGetLock
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return GetResult{}, ErrFailedToGetLock
	}

	defer lock.Release()

	return source.Create(ctx)
}
