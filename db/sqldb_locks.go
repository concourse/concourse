package db

import (
	"hash/crc32"

	"code.cloudfoundry.org/lager"
)

func (db *SQLDB) GetLock(logger lager.Logger, taskName string) (Lock, bool, error) {
	lock := db.lockFactory.NewLock(
		logger.Session("lock"),
		lockIDForTaskName(taskName),
	)

	renewed, err := lock.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, false, nil
	}

	return lock, true, nil
}

func lockIDForTaskName(taskName string) int {
	return int(crc32.ChecksumIEEE([]byte(taskName)))
}

const BUILD_TRACKING = 1
