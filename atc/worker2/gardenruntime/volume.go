package gardenruntime

import (
	"archive/tar"
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
	"github.com/hashicorp/go-multierror"
)

const creatingVolumeRetryDelay = 1 * time.Second

// TODO
var defaultCompression = compression.NewGzipCompression()

type Volume struct {
	dbVolume db.CreatedVolume
	bcVolume baggageclaim.Volume
}

func (v Volume) Handle() string {
	return v.bcVolume.Handle()
}

func (v Volume) Path() string {
	return v.bcVolume.Path()
}

func (v Volume) COWStrategy() baggageclaim.COWStrategy {
	return baggageclaim.COWStrategy{
		Parent: v.bcVolume,
	}
}

func (v Volume) StreamOut(ctx context.Context, path string, encoding runtime.Encoding) (io.ReadCloser, error) {
	return v.bcVolume.StreamOut(ctx, path, baggageclaim.Encoding(encoding))
}

func (v Volume) StreamIn(ctx context.Context, path string, encoding runtime.Encoding, reader io.Reader) error {
	return v.bcVolume.StreamIn(ctx, path, baggageclaim.Encoding(encoding), reader)
}

func (worker Worker) LookupVolume(logger lager.Logger, handle string) (runtime.Volume, bool, error) {
	_, createdVolume, err := worker.db.VolumeRepo.FindVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-db", err)
		return Volume{}, false, err
	}

	if createdVolume == nil {
		return Volume{}, false, nil
	}

	bcVolume, found, err := worker.bcClient.LookupVolume(logger, handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return Volume{}, false, err
	}

	if !found {
		return Volume{}, false, nil
	}

	return Volume{bcVolume: bcVolume, dbVolume: createdVolume}, true, nil
}

func (worker *Worker) findOrCreateVolumeForContainer(
	logger lager.Logger,
	volumeSpec runtime.VolumeSpec,
	container db.CreatingContainer,
	teamID int,
	mountPath string,
) (Volume, error) {
	return worker.findOrCreateVolume(
		logger.Session("find-or-create-volume-for-container"),
		volumeSpec,
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return worker.db.VolumeRepo.FindContainerVolume(teamID, worker.Name(), container, mountPath)
		},
		func() (db.CreatingVolume, error) {
			return worker.db.VolumeRepo.CreateContainerVolume(teamID, worker.Name(), container, mountPath)
		},
	)
}

func (worker *Worker) findOrCreateCOWVolumeForContainer(
	logger lager.Logger,
	privileged bool,
	container db.CreatingContainer,
	parent Volume,
	teamID int,
	mountPath string,
) (Volume, error) {
	return worker.findOrCreateVolume(
		logger.Session("find-or-create-cow-volume-for-container"),
		runtime.VolumeSpec{
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
	logger lager.Logger,
	volumeSpec runtime.VolumeSpec,
	teamID int,
	resourceTypeName string,
) (Volume, error) {
	logger = logger.Session("find-or-create-volume-for-base-resource-type", lager.Data{
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
		logger,
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
	logger lager.Logger,
	teamID int,
	jobID int,
	stepName string,
	path string,
) (Volume, bool, error) {
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

	bcVolume, found, err := worker.bcClient.LookupVolume(logger, dbVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return Volume{}, false, err
	}
	if !found {
		return Volume{}, false, nil
	}

	return Volume{bcVolume: bcVolume, dbVolume: dbVolume}, true, nil
}
func (worker *Worker) findOrCreateVolumeForResourceCerts(logger lager.Logger) (Volume, bool, error) {
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
		logger.Session("find-or-create-volume-for-resource-certs"),
		runtime.VolumeSpec{
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
	logger lager.Logger,
	volumeSpec runtime.VolumeSpec,
	findVolumeFunc func() (db.CreatingVolume, db.CreatedVolume, error),
	createVolumeFunc func() (db.CreatingVolume, error),
) (Volume, error) {
	creatingVolume, createdVolume, err := findVolumeFunc()
	if err != nil {
		logger.Error("failed-to-find-volume-in-db", err)
		return Volume{}, err
	}

	if createdVolume != nil {
		logger = logger.WithData(lager.Data{"volume": createdVolume.Handle()})

		bcVolume, bcVolumeFound, err := worker.bcClient.LookupVolume(logger.Session("lookup-volume"), createdVolume.Handle())
		if err != nil {
			logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
			return Volume{}, err
		}
		if !bcVolumeFound {
			logger.Info("created-volume-not-found")
			return Volume{}, CreatedVolumeNotFoundError{Handle: createdVolume.Handle(), WorkerName: createdVolume.WorkerName()}
		}

		logger.Debug("found-created-volume")
		return Volume{bcVolume: bcVolume, dbVolume: createdVolume}, nil
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
		return worker.findOrCreateVolume(logger, volumeSpec, findVolumeFunc, createVolumeFunc)
	}
	defer lock.Release()

	bcVolume, bcVolumeFound, err := worker.bcClient.LookupVolume(logger.Session("create-volume"), creatingVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
		return Volume{}, err
	}
	if bcVolumeFound {
		logger.Debug("real-volume-exists")
	} else {
		logger.Debug("creating-real-volume")

		bcVolume, err = worker.bcClient.CreateVolume(
			logger.Session("create-volume"),
			creatingVolume.Handle(),
			toBaggageclaimVolumeSpec(volumeSpec),
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

	return Volume{bcVolume: bcVolume, dbVolume: createdVolume}, nil
}

func toBaggageclaimVolumeSpec(spec runtime.VolumeSpec) baggageclaim.VolumeSpec {
	return baggageclaim.VolumeSpec{
		Strategy:   spec.Strategy,
		Privileged: spec.Privileged,
		Properties: baggageclaim.VolumeProperties(spec.Properties),
	}
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

// TODO: find a home for this
func streamFile(ctx context.Context, volume runtime.Volume, path string) (io.ReadCloser, error) {
	out, err := volume.StreamOut(ctx, path, runtime.Encoding(defaultCompression.Encoding()))
	if err != nil {
		return nil, err
	}

	compressionReader, err := defaultCompression.NewReader(out)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(compressionReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, err
	}

	return fileReadMultiCloser{
		Reader: tarReader,
		closers: []io.Closer{
			out,
			compressionReader,
		},
	}, nil
}

// TODO: ...and this
// TODO: also handle P2P streaming
func stream(ctx context.Context, srcWorker string, src runtime.Volume, dst runtime.Volume) error {
	logger := lagerctx.FromContext(ctx).Session("stream-to")
	logger.Info("start")
	defer logger.Info("end")

	_, outSpan := tracing.StartSpan(ctx, "volume.StreamOut", tracing.Attrs{
		"origin-volume": src.Handle(),
		"origin-worker": srcWorker,
	})
	defer outSpan.End()
	out, err := src.StreamOut(ctx, ".", runtime.Encoding(defaultCompression.Encoding()))

	if err != nil {
		tracing.End(outSpan, err)
		return err
	}

	defer out.Close()

	return dst.StreamIn(ctx, ".", runtime.Encoding(defaultCompression.Encoding()), out)
}

type fileReadMultiCloser struct {
	io.Reader
	closers []io.Closer
}

func (frc fileReadMultiCloser) Close() error {
	var closeErrors error

	for _, closer := range frc.closers {
		err := closer.Close()
		if err != nil {
			closeErrors = multierror.Append(closeErrors, err)
		}
	}

	return closeErrors
}
