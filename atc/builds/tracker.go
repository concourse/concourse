package builds

import (
	"context"
	"fmt"
	"sync"

	"code.cloudfoundry.org/lager/v3"

	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/util"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Engine
type Engine interface {
	NewBuild(db.Build) Runnable

	Drain(context.Context)
}

//counterfeiter:generate . Runnable
type Runnable interface {
	Run(context.Context)
}

func NewTracker(
	logger lager.Logger,
	buildFactory db.BuildFactory,
	engine Engine,
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
	engine       Engine

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
		bt.trackBuild(logger, b)
	}

	return nil
}

func (bt *Tracker) Drain(ctx context.Context) {
	bt.engine.Drain(ctx)
}

func (bt *Tracker) trackBuild(logger lager.Logger, b db.Build) {
	var id string
	if b.ID() != 0 {
		id = fmt.Sprintf("build-%d", b.ID())
	} else {
		id = fmt.Sprintf("resource-%d", b.ResourceID())
	}

	if _, exists := bt.running.LoadOrStore(id, true); exists {
		return
	}

	go func(build db.Build, id string) {
		loggerData := build.LagerData()
		defer func() {
			err := util.DumpPanic(recover(), "tracking build %d", build.ID())
			if err != nil {
				logger.Error("panic-in-tracker-build-run", err)

				build.Finish(db.BuildStatusErrored)
			}
		}()

		defer bt.running.Delete(id)

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
	}(b, id)
}

func (bt *Tracker) trackInMemoryBuilds(logger lager.Logger) {
	logger = logger.Session("tracker-imb")
	logger.Debug("start")
	defer logger.Debug("end")

	for {
		select {
		case b := <-bt.checkBuildsChan:
			if b == nil {
				return
			}
			logger.Debug("received-in-memory-build", b.LagerData())
			bt.trackBuild(logger, b)
		}
	}
}
