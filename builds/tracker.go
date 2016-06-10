package builds

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . TrackerDB

type TrackerDB interface {
	GetAllStartedBuilds() ([]db.BuildDB, error)
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
	buildDBs, err := bt.trackerDB.GetAllStartedBuilds()
	if err != nil {
		bt.logger.Error("failed-to-lookup-started-builds", err)
	}

	for _, buildDB := range buildDBs {
		tLog := bt.logger.Session("track", lager.Data{
			"build": buildDB.GetID(),
		})

		engineBuild, err := bt.engine.LookupBuild(tLog, buildDB)
		if err != nil {
			tLog.Error("failed-to-lookup-build", err)

			err := buildDB.MarkAsFailed(err)
			if err != nil {
				tLog.Error("failed-to-mark-build-as-errored", err)
			}

			continue
		}

		go engineBuild.Resume(tLog)
	}
}
