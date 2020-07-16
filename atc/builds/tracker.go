package builds

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"sync"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/metric"
)

func NewTracker(
	buildFactory db.BuildFactory,
	engine engine.Engine,
) *Tracker {
	return &Tracker{
		buildFactory: buildFactory,
		engine:       engine,
		running:      &sync.Map{},
	}
}

type Tracker struct {
	logger lager.Logger

	buildFactory db.BuildFactory
	engine       engine.Engine

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
		if _, exists := bt.running.LoadOrStore(b.ID(), true); !exists {
			go func(build db.Build) {
				loggerData := lager.Data{
					"build":    strconv.Itoa(build.ID()),
					"pipeline": build.PipelineName(),
					"job":      build.JobName(),
				}
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic in tracker build run %s: %v", loggerData, r)

						fmt.Fprintf(os.Stderr, "%s\n %s\n", err.Error(), string(debug.Stack()))
						logger.Error("panic-in-tracker-build-run", err)

						build.Finish(db.BuildStatusErrored)
					}
				}()

				defer bt.running.Delete(build.ID())

				metric.BuildsRunning.Inc()
				defer metric.BuildsRunning.Dec()

				bt.engine.NewBuild(build).Run(
					lagerctx.NewContext(
						context.Background(),
						logger.Session("run", loggerData),
					),
				)
			}(b)
		}
	}

	return nil
}

func (bt *Tracker) Drain(ctx context.Context) {
	bt.engine.Drain(ctx)
}
