package builds

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
)

//go:generate counterfeiter . TrackerDB

type TrackerDB interface {
	GetAllStartedBuilds() ([]db.Build, error)
}

func NewTracker(
	logger lager.Logger,

	trackerDB TrackerDB,
	engine engine.Engine,
) *Tracker {
	return &Tracker{
		logger:    logger,
		trackerDB: trackerDB,
		engine:    engine,
	}
}

type Tracker struct {
	logger lager.Logger

	trackerDB TrackerDB
	engine    engine.Engine
}

func (bt *Tracker) Track() {
	bt.logger.Debug("start")
	defer bt.logger.Debug("done")
	builds, err := bt.trackerDB.GetAllStartedBuilds()
	if err != nil {
		bt.logger.Error("failed-to-lookup-started-builds", err)
	}

	for _, build := range builds {
		tLog := bt.logger.Session("track", lager.Data{
			"build": build.ID(),
		})

		engineBuild, err := bt.engine.LookupBuild(tLog, build)
		if err != nil {
			tLog.Error("failed-to-lookup-build", err)

			err := build.MarkAsFailed(err)
			if err != nil {
				tLog.Error("failed-to-mark-build-as-errored", err)
			}

			continue
		}

		go engineBuild.Resume(tLog)
	}
}
