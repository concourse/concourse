package pipelines

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/scheduler/inputmapper"
	"github.com/concourse/atc/scheduler/inputmapper/inputconfig"
	"github.com/concourse/atc/scheduler/maxinflight"
)

//go:generate counterfeiter . RadarSchedulerFactory

type RadarSchedulerFactory interface {
	BuildScanRunnerFactory(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline, externalURL string) radar.ScanRunnerFactory
	BuildScheduler(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline, externalURL string) scheduler.BuildScheduler
}

type radarSchedulerFactory struct {
	resourceFactory resource.ResourceFactory
	interval        time.Duration
	engine          engine.Engine
}

func NewRadarSchedulerFactory(
	resourceFactory resource.ResourceFactory,
	interval time.Duration,
	engine engine.Engine,
) RadarSchedulerFactory {
	return &radarSchedulerFactory{
		resourceFactory: resourceFactory,
		interval:        interval,
		engine:          engine,
	}
}

func (rsf *radarSchedulerFactory) BuildScanRunnerFactory(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline, externalURL string) radar.ScanRunnerFactory {
	return radar.NewScanRunnerFactory(rsf.resourceFactory, rsf.interval, pipelineDB, dbPipeline, clock.NewClock(), externalURL)
}

func (rsf *radarSchedulerFactory) BuildScheduler(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline, externalURL string) scheduler.BuildScheduler {
	scanner := radar.NewResourceScanner(
		clock.NewClock(),
		rsf.resourceFactory,
		rsf.interval,
		pipelineDB,
		dbPipeline,
		externalURL,
	)
	inputMapper := inputmapper.NewInputMapper(
		pipelineDB,
		inputconfig.NewTransformer(pipelineDB),
	)
	return &scheduler.Scheduler{
		DB:          pipelineDB,
		InputMapper: inputMapper,
		BuildStarter: scheduler.NewBuildStarter(
			pipelineDB,
			maxinflight.NewUpdater(pipelineDB),
			factory.NewBuildFactory(
				pipelineDB.GetPipelineID(),
				atc.NewPlanFactory(time.Now().Unix()),
			),
			scanner,
			inputMapper,
			rsf.engine,
		),
		Scanner: scanner,
	}
}
