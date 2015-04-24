package builds

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . TrackerDB

type TrackerDB interface {
	GetAllStartedBuilds() ([]db.Build, error)
	ErrorBuild(buildID int, err error) error
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
	bt.logger.Info("start")
	defer bt.logger.Info("done")
	builds, err := bt.trackerDB.GetAllStartedBuilds()
	if err != nil {
		bt.logger.Error("failed-to-lookup-started-builds", err)
	}

	for _, b := range builds {
		tLog := bt.logger.Session("track", lager.Data{
			"build": b.ID,
		})

		engineBuild, err := bt.engine.LookupBuild(b)
		if err != nil {
			tLog.Error("failed-to-lookup-build", err)

			err := bt.trackerDB.ErrorBuild(b.ID, err)
			if err != nil {
				tLog.Error("failed-to-mark-build-as-errored", err)
			}

			continue
		}

		go engineBuild.Resume(tLog)
	}
}
