package pipelines

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/radar"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/concourse/concourse/atc/scheduler/inputmapper"
	"github.com/concourse/concourse/atc/scheduler/inputmapper/inputconfig"
	"github.com/concourse/concourse/atc/scheduler/maxinflight"
)

//go:generate counterfeiter . RadarSchedulerFactory

type RadarSchedulerFactory interface {
	BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables) radar.ScanRunnerFactory
	BuildScheduler(pipeline db.Pipeline, externalURL string, variables creds.Variables) scheduler.BuildScheduler
}

type radarSchedulerFactory struct {
	resourceFactory                   resource.ResourceFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	resourceTypeCheckingInterval      time.Duration
	resourceCheckingInterval          time.Duration
	engine                            engine.Engine
	workerFactory                     db.WorkerFactory
}

func NewRadarSchedulerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	engine engine.Engine,
	workerFactory db.WorkerFactory,
) RadarSchedulerFactory {
	return &radarSchedulerFactory{
		resourceFactory:                   resourceFactory,
		resourceConfigCheckSessionFactory: resourceConfigCheckSessionFactory,
		resourceTypeCheckingInterval:      resourceTypeCheckingInterval,
		resourceCheckingInterval:          resourceCheckingInterval,
		engine:                            engine,
		workerFactory:					   workerFactory,
	}
}

func (rsf *radarSchedulerFactory) BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables) radar.ScanRunnerFactory {
	return radar.NewScanRunnerFactory(rsf.resourceFactory, rsf.resourceConfigCheckSessionFactory, rsf.resourceTypeCheckingInterval, rsf.resourceCheckingInterval, dbPipeline, clock.NewClock(), externalURL, variables)
}

func (rsf *radarSchedulerFactory) BuildScheduler(pipeline db.Pipeline, externalURL string, variables creds.Variables) scheduler.BuildScheduler {

	resourceTypeScanner := radar.NewResourceTypeScanner(
		clock.NewClock(),
		rsf.resourceFactory,
		rsf.resourceConfigCheckSessionFactory,
		rsf.resourceTypeCheckingInterval,
		pipeline,
		externalURL,
		variables,
	)

	scanner := radar.NewResourceScanner(
		clock.NewClock(),
		rsf.resourceFactory,
		rsf.resourceConfigCheckSessionFactory,
		rsf.resourceCheckingInterval,
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
			maxinflight.NewUpdater(pipeline, rsf.workerFactory),
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
