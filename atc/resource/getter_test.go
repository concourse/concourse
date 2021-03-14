package resource_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/stretchr/testify/require"
)

func TestGetter_ResourceCacheNotPresent(t *testing.T) {
	lockFactory, clock := lockOnAttempt(1)
	resourceCacheFactory := new(dbfakes.FakeResourceCacheFactory)
	volumeRepo := new(dbfakes.FakeVolumeRepository)

	resourceGetter := resource.NewGetter(lockFactory, clock, resourceCacheFactory, volumeRepo)

	expectedResult := resource.VersionResult{
		Version:  atc.Version{"version": "v1"},
		Metadata: []atc.MetadataField{{Name: "foo", Value: "bar"}},
	}
	containerFactory, resourceVolume := containerFactory(runtimetest.ProcessStub{
		Output: expectedResult,
		Stderr: "some stderr log",
	})
	worker := runtimetest.NewWorker("worker")
	getResource := resource.Resource{Source: atc.Source{"some": "source"}}
	resourceCache := new(dbfakes.FakeUsedResourceCache)
	stderr := new(bytes.Buffer)

	ctx := context.Background()
	result, processResult, volume, err := resourceGetter.Get(ctx, worker, containerFactory, getResource, resourceCache, stderr)
	require.NoError(t, err)

	t.Run("validate process was invoked", func(t *testing.T) {
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)
		require.Equal(t, "some stderr log", stderr.String())
	})

	t.Run("validate resource volume was initialized as a cache", func(t *testing.T) {
		require.Equal(t, resourceVolume, volume)

		require.True(t, resourceVolume.ResourceCacheInitialized)
		require.Equal(t, 1, resourceCacheFactory.UpdateResourceCacheMetadataCallCount())
	})
}

func TestGetter_ResourceCacheExists(t *testing.T) {
	lockFactory, clock := neverLock()
	resourceCacheFactory := new(dbfakes.FakeResourceCacheFactory)
	volumeRepo := new(dbfakes.FakeVolumeRepository)

	resourceGetter := resource.NewGetter(lockFactory, clock, resourceCacheFactory, volumeRepo)

	cachedVolume := runtimetest.NewVolume("cached-volume")
	worker := runtimetest.NewWorker("worker").WithVolumes(cachedVolume)
	volumeRepo.FindResourceCacheVolumeReturns(cachedVolume.DBVolume_, true, nil)

	expectedMetadata := db.ResourceConfigMetadataFields{{Name: "foo", Value: "bar"}}
	resourceCacheFactory.ResourceCacheMetadataReturns(expectedMetadata, nil)

	expectedVersion := atc.Version{"version": "v1"}
	resourceCache := new(dbfakes.FakeUsedResourceCache)
	resourceCache.VersionReturns(expectedVersion)

	containerFactory, _ := containerFactory(runtimetest.ProcessStub{
		Err: "should not run",
	})
	getResource := resource.Resource{Source: atc.Source{"some": "source"}}

	ctx := context.Background()
	result, processResult, volume, err := resourceGetter.Get(ctx, worker, containerFactory, getResource, resourceCache, new(bytes.Buffer))
	require.NoError(t, err)

	t.Run("validate the result was fetched from cache", func(t *testing.T) {
		require.Equal(t, resource.VersionResult{
			Version:  expectedVersion,
			Metadata: expectedMetadata.ToATCMetadata(),
		}, result)
		require.Equal(t, 0, processResult.ExitStatus)
		require.Equal(t, cachedVolume, volume)
	})
}

func TestGetter_ResourceCacheInDBButMissingFromWorker(t *testing.T) {
	lockFactory, clock := lockOnAttempt(1)
	resourceCacheFactory := new(dbfakes.FakeResourceCacheFactory)
	volumeRepo := new(dbfakes.FakeVolumeRepository)

	resourceGetter := resource.NewGetter(lockFactory, clock, resourceCacheFactory, volumeRepo)

	worker := runtimetest.NewWorker("worker")
	dbVolume := new(dbfakes.FakeCreatedVolume)
	dbVolume.HandleReturns("missing-from-worker")
	volumeRepo.FindResourceCacheVolumeReturns(dbVolume, true, nil)

	expectedResult := resource.VersionResult{
		Version:  atc.Version{"version": "v1"},
		Metadata: []atc.MetadataField{{Name: "foo", Value: "bar"}},
	}
	containerFactory, _ := containerFactory(runtimetest.ProcessStub{
		Output: expectedResult,
	})
	getResource := resource.Resource{Source: atc.Source{"some": "source"}}
	resourceCache := new(dbfakes.FakeUsedResourceCache)

	ctx := context.Background()
	result, _, _, err := resourceGetter.Get(ctx, worker, containerFactory, getResource, resourceCache, new(bytes.Buffer))
	require.NoError(t, err)

	t.Run("validate get step is run", func(t *testing.T) {
		require.Equal(t, expectedResult, result)
	})
}

func TestGetter_LoopsUntilLockAcquired(t *testing.T) {
	lockFactory, clock := lockOnAttempt(10)
	resourceCacheFactory := new(dbfakes.FakeResourceCacheFactory)
	volumeRepo := new(dbfakes.FakeVolumeRepository)

	resourceGetter := resource.NewGetter(lockFactory, clock, resourceCacheFactory, volumeRepo)

	worker := runtimetest.NewWorker("worker")
	dbVolume := new(dbfakes.FakeCreatedVolume)
	dbVolume.HandleReturns("missing-from-worker")
	volumeRepo.FindResourceCacheVolumeReturns(dbVolume, true, nil)

	expectedResult := resource.VersionResult{
		Version:  atc.Version{"version": "v1"},
		Metadata: []atc.MetadataField{{Name: "foo", Value: "bar"}},
	}
	containerFactory, _ := containerFactory(runtimetest.ProcessStub{
		Output: expectedResult,
	})
	getResource := resource.Resource{Source: atc.Source{"some": "source"}}
	resourceCache := new(dbfakes.FakeUsedResourceCache)

	ctx := context.Background()
	result, _, _, err := resourceGetter.Get(ctx, worker, containerFactory, getResource, resourceCache, new(bytes.Buffer))
	require.NoError(t, err)

	t.Run("validate get step is run", func(t *testing.T) {
		require.Equal(t, expectedResult, result)
	})
}

func lockOnAttempt(attemptNumber int) (*lockfakes.FakeLockFactory, *fakeclock.FakeClock) {
	fakeClock := fakeclock.NewFakeClock(time.Unix(0, 123))
	fakeLockFactory := new(lockfakes.FakeLockFactory)
	fakeLockFactory.AcquireStub = func(lager.Logger, lock.LockID) (lock.Lock, bool, error) {
		attemptNumber--
		fakeClock.Increment(resource.GetResourceLockInterval)
		if attemptNumber <= 0 {
			return new(lockfakes.FakeLock), true, nil
		}
		return nil, false, nil
	}

	return fakeLockFactory, fakeClock
}

func neverLock() (*lockfakes.FakeLockFactory, *fakeclock.FakeClock) {
	fakeLockFactory := new(lockfakes.FakeLockFactory)
	fakeLockFactory.AcquireReturns(nil, false, errors.New("expected not to acquire a lock"))
	fakeClock := fakeclock.NewFakeClock(time.Unix(0, 123))
	return fakeLockFactory, fakeClock
}

func containerFactory(process runtimetest.ProcessStub) (func(context.Context) (runtime.Container, []runtime.VolumeMount, error), *runtimetest.Volume) {
	resourceVolume := runtimetest.NewVolume("volume")
	return func(_ context.Context) (runtime.Container, []runtime.VolumeMount, error) {
		return runtimetest.NewContainer().
				WithProcess(
					runtime.ProcessSpec{
						ID:   "resource",
						Path: "/opt/resource/in",
						Args: []string{"/tmp/build/get"},
					},
					process,
				),
			[]runtime.VolumeMount{
				{
					MountPath: "/tmp/build/get",
					Volume:    resourceVolume,
				},
			},
			nil
	}, resourceVolume
}
