package resource

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/runtime"
)

const GetResourceLockInterval = 5 * time.Second

//go:generate counterfeiter . Getter

type Getter interface {
	Get(
		context.Context,
		runtime.Worker,
		func(ctx context.Context) (runtime.Container, []runtime.VolumeMount, error),
		Resource,
		db.UsedResourceCache,
		io.Writer,
	) (VersionResult, runtime.ProcessResult, runtime.Volume, error)
}

type getter struct {
	lockFactory          lock.LockFactory
	clock                clock.Clock
	resourceCacheFactory db.ResourceCacheFactory
	volumeRepo           db.VolumeRepository
}

func NewGetter(lockFactory lock.LockFactory, clock clock.Clock, resourceCacheFactory db.ResourceCacheFactory, volumeRepo db.VolumeRepository) Getter {
	return getter{
		lockFactory:          lockFactory,
		clock:                clock,
		resourceCacheFactory: resourceCacheFactory,
		volumeRepo:           volumeRepo,
	}
}

func (g getter) Get(
	ctx context.Context,
	worker runtime.Worker,
	containerFactory func(ctx context.Context) (runtime.Container, []runtime.VolumeMount, error),
	getResource Resource,
	resourceCache db.UsedResourceCache,
	stderr io.Writer,
) (VersionResult, runtime.ProcessResult, runtime.Volume, error) {
	logger := lagerctx.FromContext(ctx)

	ticker := g.clock.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	result, processResult, volume, ok, err := g.getUnderLock(ctx, logger, worker, containerFactory, getResource, resourceCache, stderr)
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, nil, err
	}
	if ok {
		return result, processResult, volume, nil
	}

	for {
		select {
		case <-ctx.Done():
			return VersionResult{}, runtime.ProcessResult{}, nil, ctx.Err()

		case <-ticker.C():
			result, processResult, volume, ok, err := g.getUnderLock(ctx, logger, worker, containerFactory, getResource, resourceCache, stderr)
			if err != nil {
				return VersionResult{}, runtime.ProcessResult{}, nil, err
			}
			if ok {
				return result, processResult, volume, nil
			}
		}
	}
}

func (g getter) getUnderLock(
	ctx context.Context,
	logger lager.Logger,
	worker runtime.Worker,
	containerFactory func(ctx context.Context) (runtime.Container, []runtime.VolumeMount, error),
	getResource Resource,
	resourceCache db.UsedResourceCache,
	stderr io.Writer,
) (VersionResult, runtime.ProcessResult, runtime.Volume, bool, error) {
	result, processResult, volume, found, err := g.findCache(ctx, logger, worker, resourceCache)
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, nil, false, err
	}
	if found {
		return result, processResult, volume, true, nil
	}

	lockName := lockName(getResource, worker.Name())
	lockLogger := logger.Session("lock", lager.Data{"lock-name": lockName})
	lock, acquired, err := g.lockFactory.Acquire(lockLogger, lock.NewTaskLockID(lockName))
	if err != nil {
		lockLogger.Error("failed-to-get-lock", err)
		// not returning error for consistency with prior behaviour - we just
		// retry after GetResourceLockInterval
		return VersionResult{}, runtime.ProcessResult{}, nil, false, nil
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return VersionResult{}, runtime.ProcessResult{}, nil, false, nil
	}

	defer lock.Release()

	result, processResult, volume, err = g.getAndCreateCache(ctx, logger, worker, containerFactory, getResource, resourceCache, stderr)
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, nil, false, err
	}

	return result, processResult, volume, true, nil
}

func (g getter) findCache(
	ctx context.Context,
	logger lager.Logger,
	worker runtime.Worker,
	resourceCache db.UsedResourceCache,
) (VersionResult, runtime.ProcessResult, runtime.Volume, bool, error) {
	logger = logger.Session("find")

	dbVolume, found, err := g.volumeRepo.FindResourceCacheVolume(worker.Name(), resourceCache)
	if err != nil {
		logger.Error("failed-to-find-resource-cache-volume", err)
		return VersionResult{}, runtime.ProcessResult{}, nil, false, err
	}
	if !found {
		return VersionResult{}, runtime.ProcessResult{}, nil, false, nil
	}

	volume, found, err := worker.LookupVolume(logger, dbVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-resource-cache-volume-on-worker", err)
		return VersionResult{}, runtime.ProcessResult{}, nil, false, err
	}
	if !found {
		return VersionResult{}, runtime.ProcessResult{}, nil, false, nil
	}

	metadata, err := g.resourceCacheFactory.ResourceCacheMetadata(resourceCache)
	if err != nil {
		logger.Error("failed-to-get-resource-cache-metadata", err)
		return VersionResult{}, runtime.ProcessResult{}, nil, false, err
	}

	result := VersionResult{Version: resourceCache.Version(), Metadata: metadata.ToATCMetadata()}
	logger.Debug("found-initialized-versioned-source", lager.Data{
		"version":  result.Version,
		"metadata": result.Metadata,
	})

	return result, runtime.ProcessResult{ExitStatus: 0}, volume, true, nil
}

func (g getter) getAndCreateCache(
	ctx context.Context,
	logger lager.Logger,
	worker runtime.Worker,
	containerFactory func(ctx context.Context) (runtime.Container, []runtime.VolumeMount, error),
	getResource Resource,
	resourceCache db.UsedResourceCache,
	stderr io.Writer,
) (VersionResult, runtime.ProcessResult, runtime.Volume, error) {
	result, processResult, volume, found, err := g.findCache(ctx, logger, worker, resourceCache)
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, nil, err
	}
	if found {
		return result, processResult, volume, nil
	}

	logger = logger.Session("create")

	container, mounts, err := containerFactory(lagerctx.NewContext(ctx, logger))
	if err != nil {
		logger.Error("failed-to-create-container", err)
		return VersionResult{}, runtime.ProcessResult{}, nil, err
	}

	result, processResult, err = getResource.Get(ctx, container, stderr)
	if err != nil {
		logger.Error("failed-to-get-resource", err)
		return VersionResult{}, runtime.ProcessResult{}, nil, err
	}

	if processResult.ExitStatus != 0 {
		return result, processResult, nil, nil
	}

	volume = resourceMountVolume(mounts)

	if err := volume.InitializeResourceCache(logger, resourceCache); err != nil {
		return VersionResult{}, runtime.ProcessResult{}, nil, err
	}

	if err := g.resourceCacheFactory.UpdateResourceCacheMetadata(resourceCache, result.Metadata); err != nil {
		logger.Error("failed-to-update-resource-cache-metadata", err)
		return VersionResult{}, runtime.ProcessResult{}, nil, err
	}

	return result, processResult, volume, nil
}

func resourceMountVolume(mounts []runtime.VolumeMount) runtime.Volume {
	for _, mnt := range mounts {
		if mnt.MountPath == ResourcesDir("get") {
			return mnt.Volume
		}
	}
	return nil
}

func lockName(getResource Resource, workerName string) string {
	resourceJSON, _ := getResource.Signature()
	data := append(resourceJSON, []byte(workerName)...)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
