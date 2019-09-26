package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/concourse/concourse/atc/resource"

	"github.com/concourse/concourse/atc/runtime"

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
		processSpec runtime.ProcessSpec,
		resource resource.Resource,
		resourceTypes atc.VersionedResourceTypes,
		source atc.Source,
		params atc.Params,
		owner db.ContainerOwner,
		resourceDir string,
		imageFetchingDelegate ImageFetchingDelegate,
		cache db.UsedResourceCache,
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
	resourceTypes atc.VersionedResourceTypes,
	source atc.Source,
	params atc.Params,
	owner db.ContainerOwner,
	resourceDir string,
	imageFetchingDelegate ImageFetchingDelegate,
	cache db.UsedResourceCache,
) (GetResult, Volume, error) {
	result := GetResult{}
	var volume Volume
	containerSpec.Outputs = map[string]string{
		"resource": resourceDir,
	}

	fetchSource := f.fetchSourceFactory.NewFetchSource(logger, gardenWorker, source, params, owner, resourceDir, cache, resource, resourceTypes, containerSpec, processSpec, containerMetadata, imageFetchingDelegate)

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	// figure out the lockname earlier, because we have all the info
	lockType := resourceInstanceLockID{
		Type:       containerSpec.ImageSpec.ResourceType,
		Version:    cache.Version(),
		Source:     source,
		Params:     params,
		WorkerName: gardenWorker.Name(),
	}

	lockName, err := lockName(lockType)
	if err != nil {
		return result, nil, err
	}

	result, volume, err = f.fetchWithLock(ctx, logger, fetchSource, imageFetchingDelegate.Stdout(), cache, lockName)
	if err != ErrFailedToGetLock {
		fmt.Printf("=== fetcher->Fetch: got error but not ErrFailedToGetLock ==== err: %#v\n\n", err)
		return result, nil, err
	}

	for {
		select {
		case <-ticker.C():
			//TODO this is called redundantly?
			result, volume, err = f.fetchWithLock(ctx, logger, fetchSource, imageFetchingDelegate.Stdout(), cache, lockName)
			if err != nil {
				if err == ErrFailedToGetLock {
					break
				}
				return result, nil, err
			}

			fmt.Printf("=== fetcher->Fetch: got NO err\n")
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

type resourceInstanceLockID struct {
	Type       string      `json:"type,omitempty"`
	Version    atc.Version `json:"version,omitempty"`
	Source     atc.Source  `json:"source,omitempty"`
	Params     atc.Params  `json:"params,omitempty"`
	WorkerName string      `json:"worker_name,omitempty"`
}

func lockName(id resourceInstanceLockID) (string, error) {
	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
}
