package scheduler

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . TrackerDB
type TrackerDB interface {
	SaveBuildStatus(buildID int, status db.Status) error
}

type tracker struct {
	logger lager.Logger

	engine engine.Engine
	db     TrackerDB
	locker Locker
}

func NewTracker(logger lager.Logger, engine engine.Engine, db TrackerDB, locker Locker) BuildTracker {
	return &tracker{
		logger: logger,

		engine: engine,
		db:     db,
		locker: locker,
	}
}

func (tracker *tracker) TrackBuild(buildModel db.Build) error {
	lock, err := tracker.locker.AcquireWriteLockImmediately([]db.NamedLock{db.BuildTrackingLock(buildModel.ID)})
	if err != nil {
		return nil
	}

	defer lock.Release()

	tLog := tracker.logger.Session("tracking", lager.Data{
		"build": buildModel.ID,
	})

	tLog.Info("start")
	defer tLog.Info("done")

	build, err := tracker.engine.LookupBuild(buildModel)
	if err != nil {
		tLog.Info("saving-untrackable-build-as-errored")

		err := tracker.db.SaveBuildStatus(buildModel.ID, db.StatusErrored)
		if err != nil {
			tLog.Error("failed-to-save-untrackable-build-as-errored", err)
			return err
		}

		return nil
	}

	return build.Resume(tLog)
}
