package builds

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
)

func NewTracker(
	logger lager.Logger,

	buildFactory db.BuildFactory,
	engine engine.Engine,
) *Tracker {
	return &Tracker{
		logger:       logger,
		buildFactory: buildFactory,
		engine:       engine,
	}
}

type Tracker struct {
	logger lager.Logger

	buildFactory db.BuildFactory
	engine       engine.Engine
}

func (bt *Tracker) Track() {
	tLog := bt.logger.Session("track")

	tLog.Debug("start")
	defer tLog.Debug("done")
	builds, err := bt.buildFactory.GetAllStartedBuilds()
	if err != nil {
		tLog.Error("failed-to-lookup-started-builds", err)
	}

	for _, build := range builds {
		btLog := tLog.WithData(lager.Data{
			"build":    build.ID(),
			"pipeline": build.PipelineName(),
			"job":      build.JobName(),
		})

		engineBuild, err := bt.engine.LookupBuild(btLog, build)
		if err != nil {
			btLog.Error("failed-to-lookup-build", err)

			err := build.FinishWithError(err)
			if err != nil {
				btLog.Error("failed-to-mark-build-as-errored", err)
			}

			continue
		}

		go engineBuild.Resume(btLog)
	}
}

func (bt *Tracker) Release() {
	rLog := bt.logger.Session("release")
	rLog.Debug("start")
	defer rLog.Debug("done")

	bt.engine.ReleaseAll(rLog)
}
