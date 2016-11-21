package db

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
)

func (db *SQLDB) GetTaskLock(logger lager.Logger, taskName string) (lock.Lock, bool, error) {
	lock := db.lockFactory.NewLock(
		logger.Session("lock"),
		lock.NewTaskLockID(taskName),
	)

	acquired, err := lock.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return lock, true, nil
}

func (db *SQLDB) AcquireVolumeCreatingLock(logger lager.Logger, volumeID int) (lock.Lock, bool, error) {
	lock := db.lockFactory.NewLock(
		logger.Session("volume-creating-lock"),
		lock.NewVolumeCreatingLockID(volumeID),
	)

	acquired, err := lock.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return lock, true, nil
}
