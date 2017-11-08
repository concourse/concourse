package pipelines

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
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
	BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables) radar.ScanRunnerFactory
	BuildScheduler(pipeline db.Pipeline, externalURL string, variables creds.Variables) scheduler.BuildScheduler
}

type radarSchedulerFactory struct {
	resourceFactory                   resource.ResourceFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	interval                          time.Duration
	engine                            engine.Engine
}

func NewRadarSchedulerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory,
	interval time.Duration,
	engine engine.Engine,
) RadarSchedulerFactory {
	return &radarSchedulerFactory{
		resourceFactory:                   resourceFactory,
		resourceConfigCheckSessionFactory: resourceConfigCheckSessionFactory,
		interval: interval,
		engine:   engine,
	}
}

func (rsf *radarSchedulerFactory) BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables) radar.ScanRunnerFactory {
	return radar.NewScanRunnerFactory(rsf.resourceFactory, rsf.resourceConfigCheckSessionFactory, rsf.interval, dbPipeline, clock.NewClock(), externalURL, variables)
}

func (rsf *radarSchedulerFactory) BuildScheduler(pipeline db.Pipeline, externalURL string, variables creds.Variables) scheduler.BuildScheduler {

	resourceTypeScanner := radar.NewResourceTypeScanner(
		clock.NewClock(),
		rsf.resourceFactory,
		rsf.resourceConfigCheckSessionFactory,
		rsf.interval,
		pipeline,
		externalURL,
		variables,
	)

	scanner := radar.NewResourceScanner(
		clock.NewClock(),
		rsf.resourceFactory,
		rsf.resourceConfigCheckSessionFactory,
		rsf.interval,
		pipeline,
		externalURL,
		variables,
		resourceTypeScanner,
	)

	inputMapper := inputmapper.NewInputMapper(
		pipeline,
		inputconfig.NewTransformer(pipeline),
	)
	return &scheduler.Scheduler{
		Pipeline:    pipeline,
		InputMapper: inputMapper,
		BuildStarter: scheduler.NewBuildStarter(
			pipeline,
			maxinflight.NewUpdater(pipeline),
			factory.NewBuildFactory(
				pipeline.ID(),
				atc.NewPlanFactory(time.Now().Unix()),
			),
			scanner,
			inputMapper,
			rsf.engine,
		),
		Scanner: scanner,
	}
}
