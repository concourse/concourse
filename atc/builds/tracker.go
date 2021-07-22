package builds

import (
	"code.cloudfoundry.org/lager"
	"context"
	"sync"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/util"
)

func NewTracker(
	logger lager.Logger,
	buildFactory db.BuildFactory,
	engine engine.Engine,
	checkBuildsChan <-chan db.Build,
) *Tracker {
	tracker := &Tracker{
		buildFactory:    buildFactory,
		engine:          engine,
		running:         &sync.Map{},
		checkBuildsChan: checkBuildsChan,
	}
	go tracker.trackInMemoryBuilds(logger)
	return tracker
}

type Tracker struct {
	buildFactory db.BuildFactory
	engine       engine.Engine

	checkBuildsChan <-chan db.Build

	running *sync.Map
}

func (bt *Tracker) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	logger.Debug("start")
	defer logger.Debug("done")

	builds, err := bt.buildFactory.GetAllStartedBuilds()
	if err != nil {
		logger.Error("failed-to-lookup-started-builds", err)
		return err
	}

	for _, b := range builds {
		bt.trackBuild(logger, b, true)
	}

	return nil
}

func (bt *Tracker) Drain(ctx context.Context) {
	bt.engine.Drain(ctx)
}

func (bt *Tracker) trackBuild(logger lager.Logger, b db.Build, dupCheck bool) {
	if dupCheck {
		if _, exists := bt.running.LoadOrStore(b.ID(), true); exists {
			return
		}
	}

	go func(build db.Build) {
		loggerData := build.LagerData()
		defer func() {
			err := util.DumpPanic(recover(), "tracking build %d", build.ID())
			if err != nil {
				logger.Error("panic-in-tracker-build-run", err)

				build.Finish(db.BuildStatusErrored)
			}
		}()

		defer func(dupCheck bool) {
			if dupCheck {
				bt.running.Delete(build.ID())
			}
		}(dupCheck)

		if build.Name() == db.CheckBuildName {
			metric.Metrics.CheckBuildsRunning.Inc()
			defer metric.Metrics.CheckBuildsRunning.Dec()
		} else {
			metric.Metrics.BuildsRunning.Inc()
			defer metric.Metrics.BuildsRunning.Dec()
		}

		bt.engine.NewBuild(build).Run(
			lagerctx.NewContext(
				context.Background(),
				logger.Session("run", loggerData),
			),
		)
	}(b)
}

func (bt *Tracker) trackInMemoryBuilds(logger lager.Logger) {
	logger = logger.Session("tracker-imb")
	logger.Info("start")
	defer logger.Info("end")

	for {
		select {
		case b := <-bt.checkBuildsChan:
			if b == nil {
				return
			}
			logger.Debug("received-in-memory-build", b.LagerData())
			bt.trackBuild(logger, b, false)
		}
	}
}
