package worker

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
)

const creatingVolumeRetryDelay = 1 * time.Second

//go:generate counterfeiter . VolumeClient

type VolumeClient interface {
	FindOrCreateVolumeForContainer(
		lager.Logger,
		VolumeSpec,
		db.CreatingContainer,
		int,
		string,
	) (Volume, error)
	FindOrCreateCOWVolumeForContainer(
		lager.Logger,
		VolumeSpec,
		db.CreatingContainer,
		Volume,
		int,
		string,
	) (Volume, error)
	FindOrCreateVolumeForBaseResourceType(
		lager.Logger,
		VolumeSpec,
		int,
		string,
	) (Volume, error)
	FindVolumeForResourceCache(
		lager.Logger,
		*db.UsedResourceCache,
	) (Volume, bool, error)
	FindVolumeForTaskCache(
		logger lager.Logger,
		teamID int,
		jobID int,
		stepName string,
		path string,
	) (Volume, bool, error)
	CreateVolumeForTaskCache(
		logger lager.Logger,
		volumeSpec VolumeSpec,
		teamID int,
		jobID int,
		stepName string,
		path string,
	) (Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)
}

var ErrVolumeExpiredImmediately = errors.New("volume expired immediately after saving")

type ErrCreatedVolumeNotFound struct {
	Handle string
}

func (e ErrCreatedVolumeNotFound) Error() string {
	return fmt.Sprintf("failed to find created volume in baggageclaim. Volume handle: %s", e.Handle)
}

var ErrBaseResourceTypeNotFound = errors.New("base resource type not found")

type volumeClient struct {
	baggageclaimClient              baggageclaim.Client
	lockFactory                     lock.LockFactory
	dbVolumeFactory                 db.VolumeFactory
	dbWorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory
	dbWorkerTaskCacheFactory        db.WorkerTaskCacheFactory
	clock                           clock.Clock
	dbWorker                        db.Worker
}

func NewVolumeClient(
	baggageclaimClient baggageclaim.Client,
	dbWorker db.Worker,
	clock clock.Clock,

	lockFactory lock.LockFactory,
	dbVolumeFactory db.VolumeFactory,
	dbWorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory,
	dbWorkerTaskCacheFactory db.WorkerTaskCacheFactory,
) VolumeClient {
	return &volumeClient{
		baggageclaimClient:              baggageclaimClient,
		lockFactory:                     lockFactory,
		dbVolumeFactory:                 dbVolumeFactory,
		dbWorkerBaseResourceTypeFactory: dbWorkerBaseResourceTypeFactory,
		dbWorkerTaskCacheFactory:        dbWorkerTaskCacheFactory,
		clock:    clock,
		dbWorker: dbWorker,
	}
}

func (c *volumeClient) FindOrCreateVolumeForContainer(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	container db.CreatingContainer,
	teamID int,
	mountPath string,
) (Volume, error) {
	return c.findOrCreateVolume(
		logger.Session("find-or-create-volume-for-container"),
		volumeSpec,
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return c.dbVolumeFactory.FindContainerVolume(teamID, c.dbWorker.Name(), container, mountPath)
		},
		func() (db.CreatingVolume, error) {
			return c.dbVolumeFactory.CreateContainerVolume(teamID, c.dbWorker.Name(), container, mountPath)
		},
	)
}

func (c *volumeClient) FindOrCreateCOWVolumeForContainer(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	container db.CreatingContainer,
	parent Volume,
	teamID int,
	mountPath string,
) (Volume, error) {
	return c.findOrCreateVolume(
		logger.Session("find-or-create-cow-volume-for-container"),
		volumeSpec,
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return c.dbVolumeFactory.FindContainerVolume(teamID, c.dbWorker.Name(), container, mountPath)
		},
		func() (db.CreatingVolume, error) {
			return parent.CreateChildForContainer(container, mountPath)
		},
	)
}

func (c *volumeClient) FindOrCreateVolumeForBaseResourceType(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	teamID int,
	resourceTypeName string,
) (Volume, error) {
	workerBaseResourceType, found, err := c.dbWorkerBaseResourceTypeFactory.Find(resourceTypeName, c.dbWorker)
	if err != nil {
		return nil, err
	}
	if !found {
		logger.Error("base-resource-type-not-found", ErrBaseResourceTypeNotFound, lager.Data{"resource-type-name": resourceTypeName})
		return nil, ErrBaseResourceTypeNotFound
	}

	return c.findOrCreateVolume(
		logger.Session("find-or-create-volume-for-base-resource-type"),
		volumeSpec,
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return c.dbVolumeFactory.FindBaseResourceTypeVolume(teamID, workerBaseResourceType)
		},
		func() (db.CreatingVolume, error) {
			return c.dbVolumeFactory.CreateBaseResourceTypeVolume(teamID, workerBaseResourceType)
		},
	)
}

func (c *volumeClient) FindVolumeForResourceCache(
	logger lager.Logger,
	usedResourceCache *db.UsedResourceCache,
) (Volume, bool, error) {
	dbVolume, found, err := c.dbVolumeFactory.FindResourceCacheVolume(c.dbWorker.Name(), usedResourceCache)
	if err != nil {
		logger.Error("failed-to-lookup-resource-cache-volume-in-db", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	bcVolume, found, err := c.baggageclaimClient.LookupVolume(logger, dbVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return NewVolume(bcVolume, dbVolume, c), true, nil
}

func (c *volumeClient) CreateVolumeForTaskCache(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	teamID int,
	jobID int,
	stepName string,
	path string,
) (Volume, error) {
	taskCache, err := c.dbWorkerTaskCacheFactory.FindOrCreate(jobID, stepName, path, c.dbWorker.Name())
	if err != nil {
		logger.Error("failed-to-find-or-create-task-cache-in-db", err)
		return nil, err
	}

	return c.findOrCreateVolume(
		logger.Session("find-or-create-volume-for-container"),
		volumeSpec,
		func() (db.CreatingVolume, db.CreatedVolume, error) {
			return nil, nil, nil
		},
		func() (db.CreatingVolume, error) {
			return c.dbVolumeFactory.CreateTaskCacheVolume(teamID, taskCache)
		},
	)
}

func (c *volumeClient) FindVolumeForTaskCache(
	logger lager.Logger,
	teamID int,
	jobID int,
	stepName string,
	path string,
) (Volume, bool, error) {
	taskCache, found, err := c.dbWorkerTaskCacheFactory.Find(jobID, stepName, path, c.dbWorker.Name())
	if err != nil {
		logger.Error("failed-to-lookup-task-cache-in-db", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	_, dbVolume, err := c.dbVolumeFactory.FindTaskCacheVolume(teamID, taskCache)
	if err != nil {
		logger.Error("failed-to-lookup-tasl-cache-volume-in-db", err)
		return nil, false, err
	}

	if dbVolume == nil {
		return nil, false, nil
	}

	bcVolume, found, err := c.baggageclaimClient.LookupVolume(logger, dbVolume.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return NewVolume(bcVolume, dbVolume, c), true, nil
}

func (c *volumeClient) LookupVolume(logger lager.Logger, handle string) (Volume, bool, error) {
	dbVolume, found, err := c.dbVolumeFactory.FindCreatedVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-db", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	bcVolume, found, err := c.baggageclaimClient.LookupVolume(logger, handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-bc", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return NewVolume(bcVolume, dbVolume, c), true, nil
}

func (c *volumeClient) findOrCreateVolume(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	findVolumeFunc func() (db.CreatingVolume, db.CreatedVolume, error),
	createVolumeFunc func() (db.CreatingVolume, error),
) (Volume, error) {
	creatingVolume, createdVolume, err := findVolumeFunc()
	if err != nil {
		logger.Error("failed-to-find-volume-in-db", err)
		return nil, err
	}

	if createdVolume != nil {
		logger = logger.WithData(lager.Data{"volume": createdVolume.Handle()})

		bcVolume, bcVolumeFound, err := c.baggageclaimClient.LookupVolume(
			logger.Session("lookup-volume"),
			createdVolume.Handle(),
		)
		if err != nil {
			logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
			return nil, err
		}

		if !bcVolumeFound {
			logger.Info("created-volume-not-found")
			return nil, ErrCreatedVolumeNotFound{Handle: createdVolume.Handle()}
		}

		logger.Debug("found-created-volume")

		return NewVolume(bcVolume, createdVolume, c), nil
	}

	if creatingVolume != nil {
		logger = logger.WithData(lager.Data{"volume": creatingVolume.Handle()})
		logger.Debug("found-creating-volume")
	} else {
		creatingVolume, err = createVolumeFunc()
		if err != nil {
			logger.Error("failed-to-create-volume-in-db", err)
			return nil, err
		}

		logger = logger.WithData(lager.Data{"volume": creatingVolume.Handle()})

		logger.Debug("created-creating-volume")
	}

	lock, acquired, err := c.lockFactory.Acquire(logger, lock.NewVolumeCreatingLockID(creatingVolume.ID()))
	if err != nil {
		logger.Error("failed-to-acquire-volume-creating-lock", err)
		return nil, err
	}

	if !acquired {
		c.clock.Sleep(creatingVolumeRetryDelay)
		return c.findOrCreateVolume(logger, volumeSpec, findVolumeFunc, createVolumeFunc)
	}

	defer lock.Release()

	bcVolume, bcVolumeFound, err := c.baggageclaimClient.LookupVolume(
		logger.Session("create-volume"),
		creatingVolume.Handle(),
	)
	if err != nil {
		logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
		return nil, err
	}

	if bcVolumeFound {
		logger.Debug("real-volume-exists")
	} else {
		logger.Debug("creating-real-volume")

		bcVolume, err = c.baggageclaimClient.CreateVolume(
			logger.Session("create-volume"),
			creatingVolume.Handle(),
			volumeSpec.baggageclaimVolumeSpec(),
		)
		if err != nil {
			logger.Error("failed-to-create-volume-in-baggageclaim", err)

			_, failedErr := creatingVolume.Failed()
			if failedErr != nil {
				logger.Error("failed-to-mark-volume-as-failed", failedErr)
			}

			metric.FailedVolumes.Inc()

			return nil, err
		}

		metric.VolumesCreated.Inc()
	}

	createdVolume, err = creatingVolume.Created()
	if err != nil {
		logger.Error("failed-to-initialize-volume", err)
		return nil, err
	}

	logger.Debug("created")

	return NewVolume(bcVolume, createdVolume, c), nil
}
