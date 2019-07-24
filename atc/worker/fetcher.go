package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/concourse/concourse/atc/runtime"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/resource"
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
		resourceInstance resource.ResourceInstance,
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

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

func (f *fetcher) Fetch(
	ctx context.Context,
	logger lager.Logger,
	containerMetadata db.ContainerMetadata,
	gardenWorker Worker,
	containerSpec ContainerSpec,
	processSpec ProcessSpec,
	resourceTypes atc.VersionedResourceTypes,
	resourceInstance resource.ResourceInstance, // can we not use resource package here?
	imageFetchingDelegate ImageFetchingDelegate,
	cache db.UsedResourceCache,
	//) (resource.VersionedSource, error) {
) (GetResult, error) {
	containerSpec.Outputs = map[string]string{
		"resource": ResourcesDir("get"),
	}

	source := f.fetchSourceFactory.NewFetchSource(logger, gardenWorker, resourceInstance, cache, resourceTypes, containerSpec, processSpec, containerMetadata, imageFetchingDelegate)

	ticker := f.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	// figure out the lockname earlier, because we have all the info
	lockName, err := lockName(string(resourceInstance.ResourceType()),
		resourceInstance.Version(),
		resourceInstance.Source(),
		resourceInstance.Params(),
		gardenWorker.Name())
	if err != nil {
		return GetResult{}, err
	}

	versionedSource, err := f.fetchWithLock(ctx, logger, source, imageFetchingDelegate.Stdout(), cache, lockName)
	if err != ErrFailedToGetLock {
		return versionedSource, err
	}

	for {
		select {
		case <-ticker.C():
			//TODO this is called redundantly?
			result, err := f.fetchWithLock(ctx, logger, source, imageFetchingDelegate.Stdout(), cache, lockName)
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
	Type       string      `json:"type,omitempty"`
	Version    atc.Version `json:"version,omitempty"`
	Source     atc.Source  `json:"source,omitempty"`
	Params     atc.Params  `json:"params,omitempty"`
	WorkerName string      `json:"worker_name,omitempty"`
}

func lockName(resourceType string, version atc.Version, source atc.Source, params atc.Params, workerName string) (string, error) {
	id := &lockID{
		Type:       resourceType,
		Version:    version,
		Source:     source,
		Params:     params,
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
	volume, found, err := source.Find()
	if err != nil {
		return GetResult{}, err
	}

	if found {
		result := GetResult{
			0,
			// todo: figure out what logically should be returned for VersionResult
			runtime.VersionResult{},
			runtime.GetArtifact{VolumeHandle: volume.Handle()},
			nil,
		}
		return result, nil
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
