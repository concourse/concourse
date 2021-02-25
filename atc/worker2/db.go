package worker2

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/worker2/gardenruntime"
)

type DB struct {
	WorkerFactory                 db.WorkerFactory
	TeamFactory                   db.TeamFactory
	VolumeRepo                    db.VolumeRepository
	TaskCacheFactory              db.TaskCacheFactory
	WorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory
	LockFactory                   lock.LockFactory
}

func (db DB) ToGardenRuntimeDB() gardenruntime.DB {
	return gardenruntime.DB{
		VolumeRepo:                    db.VolumeRepo,
		TaskCacheFactory:              db.TaskCacheFactory,
		WorkerBaseResourceTypeFactory: db.WorkerBaseResourceTypeFactory,
		LockFactory:                   db.LockFactory,
	}
}
