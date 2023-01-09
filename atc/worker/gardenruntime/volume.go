package gardenruntime

import (
	"context"
	"io"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/worker/baggageclaim"
)

const creatingVolumeRetryDelay = 1 * time.Second

// Prefix used to differentiate between streamed empty volumes (that aren't
// mounted directly to containers) and the child COW volumes that we do mount
// to containers
const streamedVolumePathPrefix = "streamed-no-mount:"

type Volume struct {
	dbVolume db.CreatedVolume
	bcVolume baggageclaim.Volume
	worker   *Worker
}

func (v Volume) Handle() string {
	return v.bcVolume.Handle()
}

func (v Volume) Source() string {
	return v.dbVolume.WorkerName()
}

func (v Volume) Path() string {
	return v.bcVolume.Path()
}

func (v Volume) DBVolume() db.CreatedVolume {
	return v.dbVolume
}

func (v Volume) InitializeResourceCache(ctx context.Context, cache db.ResourceCache) (*db.UsedWorkerResourceCache, error) {
	logger := lagerctx.FromContext(ctx)
	if err := v.bcVolume.SetPrivileged(ctx, false); err != nil {
		logger.Error("failed-to-set-unprivileged", err)
		return nil, err
	}

	var uwrc *db.UsedWorkerResourceCache
	var err error
	if uwrc, err = v.dbVolume.InitializeResourceCache(cache); err != nil {
		logger.Error("failed-to-initialize-resource-cache", err)
		return nil, err
	}
	return uwrc, nil
}

func (v Volume) InitializeStreamedResourceCache(ctx context.Context, cache db.ResourceCache, sourceWorkerResourceCacheID int) (*db.UsedWorkerResourceCache, error) {
	logger := lagerctx.FromContext(ctx)
	if err := v.bcVolume.SetPrivileged(ctx, false); err != nil {
		logger.Error("failed-to-set-unprivileged", err)
		return nil, err
	}

	var uwrc *db.UsedWorkerResourceCache
	var err error
	if uwrc, err = v.dbVolume.InitializeStreamedResourceCache(cache, sourceWorkerResourceCacheID); err != nil {
		logger.Error("failed-to-initialize-resource-cache", err)
		return nil, err
	}
	return uwrc, nil
}

func (v Volume) InitializeTaskCache(ctx context.Context, jobID int, stepName string, path string, privileged bool) error {
	logger := lagerctx.FromContext(ctx)
	path = filepath.Clean(path)

	if v.dbVolume.ParentHandle() == "" {
		return v.dbVolume.InitializeTaskCache(jobID, stepName, path)
	}

	logger.Debug("creating-an-import-volume", lager.Data{"path": v.bcVolume.Path()})
	importVolume, err := v.worker.createVolumeForTaskCache(
		ctx,
		v,
		privileged,
		v.dbVolume.TeamID(),
		jobID,
		stepName,
		path,
	)
	if err != nil {
		logger.Error("failed-to-create-import-volume", err, lager.Data{"path": v.bcVolume.Path()})
		return err
	}

	return importVolume.InitializeTaskCache(ctx, jobID, stepName, path, privileged)
}

func (v Volume) COWStrategy() baggageclaim.COWStrategy {
	return baggageclaim.COWStrategy{
		Parent: v.bcVolume,
	}
}

func (v Volume) StreamOut(ctx context.Context, path string, compression compression.Compression) (io.ReadCloser, error) {
	return v.bcVolume.StreamOut(ctx, path, compression.Encoding())
}

func (v Volume) StreamIn(ctx context.Context, path string, compression compression.Compression, reader io.Reader) error {
	return v.bcVolume.StreamIn(ctx, path, compression.Encoding(), reader)
}

func (v Volume) GetStreamInP2PURL(ctx context.Context, path string) (string, error) {
	return v.bcVolume.GetStreamInP2pUrl(ctx, path)
}

func (v Volume) StreamP2POut(ctx context.Context, path string, destURL string, compression compression.Compression) error {
	return v.bcVolume.StreamP2pOut(ctx, path, destURL, compression.Encoding())
}

var _ runtime.P2PVolume = Volume{}

func (worker *Worker) newVolume(bcVolume baggageclaim.Volume, dbVolume db.CreatedVolume) Volume {
	return Volume{bcVolume: bcVolume, dbVolume: dbVolume, worker: worker}
}

func (worker *Worker) LookupVolume(ctx context.Context, handle string) (runtime.Volume, bool, error) {
	logger := lagerctx.FromContext(ctx)
	createdVolume, found, err := worker.db.VolumeRepo.FindVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-db", err)
		return Volume{}, false, err
	}

	if !found {
		return Volume{}, false, nil
	}

	bcVolume, found, err := worker.bcClient.LookupVolume(ctx, handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return Volume{}, false, err
	}

	if !found {
		return Volume{}, false, nil
	}

	return worker.newVolume(bcVolume, createdVolume), true, nil
}

func (worker *Worker) CreateVolumeForArtifact(ctx context.Context, teamID int) (runtime.Volume, db.WorkerArtifact, error) {
	logger := lagerctx.FromContext(ctx)
	creatingVolume, err := worker.db.VolumeRepo.CreateVolume(teamID, worker.Name(), db.VolumeTypeArtifact)
	if err != nil {
		logger.Error("failed-to-create-volume-in-db", err)
		return nil, nil, err
	}

	workerArtifact, err := creatingVolume.InitializeArtifact()
	if err != nil {
		logger.Error("failed-to-initialize-artifact", err)
		return nil, nil, err
	}

	bcVolume, err := worker.bcClient.CreateVolume(ctx, creatingVolume.Handle(), baggageclaim.VolumeSpec{
		Strategy: baggageclaim.EmptyStrategy{},
	})
	if err != nil {
		logger.Error("failed-to-create-volume-in-bc", err)
		return nil, nil, err
	}

	createdVolume, err := creatingVolume.Created()
	if err != nil {
		logger.Error("failed-to-mark-volume-as-created", err)
		return nil, nil, err
	}

	return worker.newVolume(bcVolume, createdVolume), workerArtifact, nil
}

func (worker *Worker) findOrCreateVolumeForContainer(
	ctx context.Context,
	volumeSpec baggageclaim.VolumeSpec,
	container db.CreatingContainer,
	teamID int,
	mountPath string,
) (Volume, error) {
	ctx = lagerctx.NewContext(ctx, lagerctx.FromContext(ctx).Session("find-or-create-volume-for-container"))
	return worker.findOrCreateVolume(
		ctx,
		volumeSpec,
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return worker.db.VolumeRepo.FindContainerVolume(teamID, worker.Name(), container, mountPath)
		},
		func() (db.CreatingVolume, error) {
			return worker.db.VolumeRepo.CreateContainerVolume(teamID, worker.Name(), container, mountPath)
		},
	)
}

func (worker *Worker) findOrCreateVolumeForStreaming(
	ctx context.Context,
	privileged bool,
	container db.CreatingContainer,
	teamID int,
	mountPath string,
) (Volume, error) {
	return worker.findOrCreateVolumeForContainer(
		ctx,
		baggageclaim.VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Privileged: privileged,
		},
		container,
		teamID,
		// we prefix the mount path to distinguish between streamed-in volumes
		// and mounted volumes.
		streamedVolumePathPrefix+mountPath,
	)
}

func (worker *Worker) findOrCreateCOWVolumeForContainer(
	ctx context.Context,
	privileged bool,
	container db.CreatingContainer,
	parent Volume,
	teamID int,
	mountPath string,
) (Volume, error) {
	ctx = lagerctx.NewContext(ctx, lagerctx.FromContext(ctx).Session("find-or-create-cow-volume-for-container"))
	return worker.findOrCreateVolume(
		ctx,
		baggageclaim.VolumeSpec{
			Strategy:   parent.COWStrategy(),
			Privileged: privileged,
		},
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return worker.db.VolumeRepo.FindContainerVolume(teamID, worker.Name(), container, mountPath)
		},
		func() (db.CreatingVolume, error) {
			return parent.dbVolume.CreateChildForContainer(container, mountPath)
		},
	)
}

func (worker *Worker) findOrCreateVolumeForBaseResourceType(
	ctx context.Context,
	volumeSpec baggageclaim.VolumeSpec,
	teamID int,
	resourceTypeName string,
) (Volume, error) {
	logger := lagerctx.FromContext(ctx).Session("find-or-create-volume-for-base-resource-type", lager.Data{
		"resource-type": resourceTypeName,
	})
	workerBaseResourceType, found, err := worker.db.WorkerBaseResourceTypeFactory.Find(resourceTypeName, worker.dbWorker)
	if err != nil {
		logger.Error("failed-to-lookup-base-resource-type", err)
		return Volume{}, err
	}
	if !found {
		logger.Error("base-resource-type-not-found", ErrBaseResourceTypeNotFound)
		return Volume{}, ErrBaseResourceTypeNotFound
	}

	return worker.findOrCreateVolume(
		lagerctx.NewContext(ctx, logger),
		volumeSpec,
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return worker.db.VolumeRepo.FindBaseResourceTypeVolume(workerBaseResourceType)
		},
		func() (db.CreatingVolume, error) {
			return worker.db.VolumeRepo.CreateBaseResourceTypeVolume(workerBaseResourceType)
		},
	)
}

func (worker *Worker) findVolumeForTaskCache(
	ctx context.Context,
	teamID int,
	jobID int,
	stepName string,
	path string,
) (Volume, bool, error) {
	logger := lagerctx.FromContext(ctx)
	usedTaskCache, found, err := worker.db.TaskCacheFactory.Find(jobID, stepName, path)
	if err != nil {
		logger.Error("failed-to-lookup-task-cache-in-db", err)
		return Volume{}, false, err
	}
	if !found {
		return Volume{}, false, nil
	}

	dbVolume, found, err := worker.db.VolumeRepo.FindTaskCacheVolume(teamID, worker.Name(), usedTaskCache)
	if err != nil {
		logger.Error("failed-to-lookup-task-cache-volume-in-db", err)
		return Volume{}, false, err
	}
	if !found {
		return Volume{}, false, nil
	}

	bcVolume, found, err := worker.bcClient.LookupVolume(ctx, dbVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return Volume{}, false, err
	}
	if !found {
		return Volume{}, false, nil
	}

	return worker.newVolume(bcVolume, dbVolume), true, nil
}

func (worker *Worker) createVolumeForTaskCache(
	ctx context.Context,
	importFromVolume Volume,
	privileged bool,
	teamID int,
	jobID int,
	stepName string,
	path string,
) (Volume, error) {
	logger := lagerctx.FromContext(ctx)
	usedTaskCache, err := worker.db.TaskCacheFactory.FindOrCreate(jobID, stepName, path)
	if err != nil {
		logger.Error("failed-to-find-or-create-task-cache-in-db", err)
		return Volume{}, err
	}

	workerTaskCache := db.WorkerTaskCache{
		WorkerName: worker.Name(),
		TaskCache:  usedTaskCache,
	}

	usedWorkerTaskCache, err := worker.db.WorkerTaskCacheFactory.FindOrCreate(workerTaskCache)
	if err != nil {
		logger.Error("failed-to-find-or-create-worker-task-cache-in-db", err)
		return Volume{}, err
	}

	return worker.findOrCreateVolume(
		lagerctx.NewContext(ctx, logger.Session("create-volume-for-task-cache")),
		baggageclaim.VolumeSpec{
			Strategy:   baggageclaim.ImportStrategy{Path: importFromVolume.Path()},
			Privileged: privileged,
		},
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return nil, nil, nil
		},
		func() (db.CreatingVolume, error) {
			return worker.db.VolumeRepo.CreateTaskCacheVolume(teamID, usedWorkerTaskCache)
		},
	)
}

// findVolumeForArtifact tries to find the Volume corresponding to the given
// Artifact, if one exists, preferring Volumes that are closer to the current
// worker. It checks for the following things, in order of preference:
//
// 1. The Artifact is a Volume on the current worker (return the input Volume)
// 2. The Artifact is a Volume on another worker, but there is an equivalent
//    resource cache Volume on the current worker (return the local resource
//    cache Volume). In this case, worker_resource_cache on the other worker
//    should be valid before the time of volumeShouldBeValidBefore. In other
//    words, if a worker resource cache has been invalidated, then later build
//    should never use it, but builds started before the cache is invalidated
//    may still use it.
// 3. The Artifact is a Volume on another worker with no local resource cache
//    Volume (return the input Volume)
// 4. The Artifact is not a Volume (return not ok)
func (worker *Worker) findVolumeForArtifact(
	ctx context.Context,
	teamID int,
	artifact runtime.Artifact,
	volumeShouldBeValidBefore time.Time,
) (runtime.Volume, bool, error) {
	logger := lagerctx.FromContext(ctx).Session("find-volume-for-artifact", lager.Data{"worker": worker.Name()})

	volume, ok := artifact.(runtime.Volume)
	if !ok {
		return nil, false, nil
	}

	// See if the volume is on the current worker. If yes, just return it.
	if volume.DBVolume().WorkerName() == worker.Name() {
		return volume, true, nil
	}

	resourceCacheID := volume.DBVolume().GetResourceCacheID()
	if resourceCacheID == 0 {
		return volume, true, nil
	}

	resourceCache, found, err := worker.db.ResourceCacheFactory.FindResourceCacheByID(resourceCacheID)
	if err != nil {
		logger.Error("failed-to-find-resource-cache-by-id", err, lager.Data{"resource-cache": resourceCacheID})
		return nil, false, err
	}
	if !found {
		logger.Debug("resource-cache-not-found", lager.Data{"resource-cache": resourceCacheID})
		return volume, true, nil
	}

	// See if the volume has an equivalent resource cache volume on the current worker,
	// If yes, return the local volume.
	dbCacheVolume, found, err := worker.db.VolumeRepo.FindResourceCacheVolume(worker.Name(), resourceCache, volumeShouldBeValidBefore)
	if err != nil {
		logger.Error("failed-to-find-resource-cache-volume", err, lager.Data{"resource-cache": resourceCacheID})
		return nil, false, err
	}
	if !found {
		logger.Info("resource-cache-volume-disappeared-from-worker", lager.Data{"resource-cache": resourceCacheID})
		return volume, true, nil
	}

	bcCacheVolume, found, err := worker.bcClient.LookupVolume(ctx, dbCacheVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return nil, false, err
	}
	if found {
		return worker.newVolume(bcCacheVolume, dbCacheVolume), true, nil
	}

	return volume, true, nil
}

func (worker *Worker) findOrCreateVolumeForResourceCerts(ctx context.Context) (Volume, bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger.Debug("finding-worker-resource-certs")
	usedResourceCerts, found, err := worker.dbWorker.ResourceCerts()
	if err != nil {
		logger.Error("failed-to-find-worker-resource-certs", err)
		return Volume{}, false, err
	}
	if !found {
		logger.Debug("worker-resource-certs-not-found")
		return Volume{}, false, nil
	}

	certsPath := worker.dbWorker.CertsPath()
	if certsPath == nil {
		logger.Debug("worker-certs-path-is-empty")
		return Volume{}, false, nil
	}

	volume, err := worker.findOrCreateVolume(
		lagerctx.NewContext(ctx, logger.Session("find-or-create-volume-for-resource-certs")),
		baggageclaim.VolumeSpec{
			Strategy: baggageclaim.ImportStrategy{
				Path:           *certsPath,
				FollowSymlinks: true,
			},
		},
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return worker.db.VolumeRepo.FindResourceCertsVolume(worker.Name(), usedResourceCerts)
		},
		func() (db.CreatingVolume, error) {
			return worker.db.VolumeRepo.CreateResourceCertsVolume(worker.Name(), usedResourceCerts)
		},
	)

	return volume, true, err
}

func (worker *Worker) findOrCreateVolume(
	ctx context.Context,
	volumeSpec baggageclaim.VolumeSpec,
	findVolumeFunc func() (db.CreatingVolume, db.CreatedVolume, error),
	createVolumeFunc func() (db.CreatingVolume, error),
) (Volume, error) {
	logger := lagerctx.FromContext(ctx)
	creatingVolume, createdVolume, err := findVolumeFunc()
	if err != nil {
		logger.Error("failed-to-find-volume-in-db", err)
		return Volume{}, err
	}

	if createdVolume != nil {
		logger = logger.WithData(lager.Data{"volume": createdVolume.Handle()})

		bcVolume, bcVolumeFound, err := worker.bcClient.LookupVolume(ctx, createdVolume.Handle())
		if err != nil {
			logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
			return Volume{}, err
		}
		if !bcVolumeFound {
			logger.Info("created-volume-not-found")
			return Volume{}, CreatedVolumeNotFoundError{Handle: createdVolume.Handle(), WorkerName: createdVolume.WorkerName()}
		}

		logger.Debug("found-created-volume")
		return worker.newVolume(bcVolume, createdVolume), nil
	}

	if creatingVolume != nil {
		logger = logger.WithData(lager.Data{"volume": creatingVolume.Handle()})
		logger.Debug("found-creating-volume")
	} else {
		creatingVolume, err = createVolumeFunc()
		if err != nil {
			logger.Error("failed-to-create-volume-in-db", err)
			return Volume{}, err
		}

		logger = logger.WithData(lager.Data{"volume": creatingVolume.Handle()})

		logger.Debug("created-creating-volume")
	}

	lock, acquired, err := worker.db.LockFactory.Acquire(logger, lock.NewVolumeCreatingLockID(creatingVolume.ID()))
	if err != nil {
		logger.Error("failed-to-acquire-volume-creating-lock", err)
		return Volume{}, err
	}
	if !acquired {
		logger.Debug("lock-already-held", lager.Data{"retry-in": creatingVolumeRetryDelay})
		time.Sleep(creatingVolumeRetryDelay)
		return worker.findOrCreateVolume(ctx, volumeSpec, findVolumeFunc, createVolumeFunc)
	}
	defer lock.Release()

	bcVolume, bcVolumeFound, err := worker.bcClient.LookupVolume(ctx, creatingVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
		return Volume{}, err
	}
	if bcVolumeFound {
		logger.Debug("real-volume-exists")
	} else {
		logger.Debug("creating-real-volume")

		bcVolume, err = worker.bcClient.CreateVolume(
			ctx,
			creatingVolume.Handle(),
			volumeSpec,
		)
		if err != nil {
			logger.Error("failed-to-create-volume-in-baggageclaim", err)

			_, failedErr := creatingVolume.Failed()
			if failedErr != nil {
				logger.Error("failed-to-mark-volume-as-failed", failedErr)
			}

			metric.Metrics.FailedVolumes.Inc()

			return Volume{}, err
		}

		metric.Metrics.VolumesCreated.Inc()
	}

	createdVolume, err = creatingVolume.Created()
	if err != nil {
		logger.Error("failed-to-initialize-volume", err)
		return Volume{}, err
	}

	logger.Debug("created")

	return worker.newVolume(bcVolume, createdVolume), nil
}

type byMountPath []runtime.VolumeMount

func (p byMountPath) Len() int {
	return len(p)
}
func (p byMountPath) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p byMountPath) Less(i, j int) bool {
	path1 := p[i].MountPath
	path2 := p[j].MountPath
	return path1 < path2
}

// for testing
func (v Volume) BaggageclaimVolume() baggageclaim.Volume {
	return v.bcVolume
}
