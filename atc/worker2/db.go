package worker2

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/worker2/gardenruntime"
)

func NewDB(
	workerFactory db.WorkerFactory,
	teamFactory db.TeamFactory,
	volumeRepo db.VolumeRepository,
	taskCacheFactory db.TaskCacheFactory,
	workerTaskCacheFactory db.WorkerTaskCacheFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	workerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory,
	lockFactory lock.LockFactory,
) DB {
	return DB{
		WorkerFactory:                 workerFactory,
		TeamFactory:                   teamFactory,
		VolumeRepo:                    volumeRepo,
		TaskCacheFactory:              taskCacheFactory,
		WorkerTaskCacheFactory:        workerTaskCacheFactory,
		ResourceCacheFactory:          resourceCacheFactory,
		WorkerBaseResourceTypeFactory: workerBaseResourceTypeFactory,
		LockFactory:                   lockFactory,
	}
}

type DB struct {
	WorkerFactory                 db.WorkerFactory
	TeamFactory                   db.TeamFactory
	VolumeRepo                    db.VolumeRepository
	TaskCacheFactory              db.TaskCacheFactory
	WorkerTaskCacheFactory        db.WorkerTaskCacheFactory
	ResourceCacheFactory          db.ResourceCacheFactory
	WorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory
	LockFactory                   lock.LockFactory
}

func (db DB) ToGardenRuntimeDB() gardenruntime.DB {
	return gardenruntime.DB{
		VolumeRepo:                    db.VolumeRepo,
		TaskCacheFactory:              db.TaskCacheFactory,
		WorkerTaskCacheFactory:        db.WorkerTaskCacheFactory,
		ResourceCacheFactory:          db.ResourceCacheFactory,
		WorkerBaseResourceTypeFactory: db.WorkerBaseResourceTypeFactory,
		LockFactory:                   db.LockFactory,
	}
}
