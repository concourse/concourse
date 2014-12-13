package scheduler

import (
	"sync"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Engine
type Engine interface {
	ResumeBuild(db.Build, lager.Logger) error
}

type tracker struct {
	logger lager.Logger

	engine Engine

	locker Locker

	trackingBuilds map[int]bool
	lock           *sync.Mutex
}

func NewTracker(logger lager.Logger, engine Engine, locker Locker) BuildTracker {
	return &tracker{
		logger: logger,

		engine: engine,

		locker: locker,

		trackingBuilds: make(map[int]bool),
		lock:           new(sync.Mutex),
	}
}

func (tracker *tracker) TrackBuild(build db.Build) error {
	lock, err := tracker.locker.AcquireWriteLockImmediately([]db.NamedLock{db.BuildTrackingLock(build.Guid)})
	if err != nil {
		return nil
	}

	defer lock.Release()

	tLog := tracker.logger.Session("tracking", lager.Data{
		"build": build.ID,
	})

	tLog.Info("start")
	defer tLog.Info("done")

	alreadyTracking := tracker.markTracking(build.ID)
	if alreadyTracking {
		tLog.Info("already-tracking")
		return nil
	}

	defer tracker.unmarkTracking(build.ID)

	return tracker.engine.ResumeBuild(build, tLog)
}

func (tracker *tracker) markTracking(buildID int) bool {
	tracker.lock.Lock()
	alreadyTracking, found := tracker.trackingBuilds[buildID]
	if !found {
		tracker.trackingBuilds[buildID] = true
	}
	tracker.lock.Unlock()

	return alreadyTracking
}

func (tracker *tracker) unmarkTracking(buildID int) {
	tracker.lock.Lock()
	delete(tracker.trackingBuilds, buildID)
	tracker.lock.Unlock()
}
