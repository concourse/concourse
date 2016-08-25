package db

import (
	"hash/crc32"
	"time"

	"code.cloudfoundry.org/lager"
)

// TODO: don't need interval

func (db *SQLDB) GetLease(logger lager.Logger, taskName string, interval time.Duration) (Lease, bool, error) {
	lease := db.leaseFactory.NewLease(
		logger.Session("lease"),
		lockIDForTaskName(taskName),
	)

	renewed, err := lease.AttemptSign()
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, false, nil
	}

	return lease, true, nil
}

func lockIDForTaskName(taskName string) int {
	return int(crc32.ChecksumIEEE([]byte(taskName)))
}

const BUILD_TRACKING = 1
