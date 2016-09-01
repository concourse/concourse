package dbgc

import (
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ReaperDB

type ReaperDB interface {
	ReapExpiredContainers() error
	ReapExpiredVolumes() error
	ReapExpiredWorkers() error
}

type DBGarbageCollector interface {
	Run() error
}

type dbGarbageCollector struct {
	logger lager.Logger
	db     ReaperDB
}

func NewDBGarbageCollector(
	logger lager.Logger,
	db ReaperDB,
) DBGarbageCollector {
	return &dbGarbageCollector{
		logger: logger,
		db:     db,
	}
}

func (c *dbGarbageCollector) Run() error {
	err := c.db.ReapExpiredContainers()
	if err != nil {
		c.logger.Error("failed-to-reap-expired-containers", err)
		return err
	}

	err = c.db.ReapExpiredVolumes()
	if err != nil {
		c.logger.Error("failed-to-reap-expired-volumes", err)
		return err
	}

	err = c.db.ReapExpiredWorkers()
	if err != nil {
		c.logger.Error("failed-to-reap-expired-workers", err)
		return err
	}

	return nil
}
