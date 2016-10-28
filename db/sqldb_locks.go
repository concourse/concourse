package db

import "code.cloudfoundry.org/lager"

func (db *SQLDB) GetTaskLock(logger lager.Logger, taskName string) (Lock, bool, error) {
	lock := db.lockFactory.NewLock(
		logger.Session("lock"),
		taskLockID(taskName),
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

func (db *SQLDB) AcquireVolumeCreatingLock(logger lager.Logger, volumeID int) (Lock, bool, error) {
	lock := db.lockFactory.NewLock(
		logger.Session("volume-creating-lock"),
		volumeCreatingLockID(volumeID),
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
