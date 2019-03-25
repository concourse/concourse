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
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . RadarSchedulerFactory

type RadarSchedulerFactory interface {
	BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables) radar.ScanRunnerFactory
	BuildScheduler(pipeline db.Pipeline, externalURL string, variables creds.Variables) scheduler.BuildScheduler
}

type radarSchedulerFactory struct {
	pool                         worker.Pool
	resourceFactory              resource.ResourceFactory
	resourceTypeCheckingInterval time.Duration
	resourceCheckingInterval     time.Duration
	engine                       engine.Engine
	strategy                     worker.ContainerPlacementStrategy

	conn db.Conn
}

func NewRadarSchedulerFactory(
	conn db.Conn,
	pool worker.Pool,
	resourceFactory resource.ResourceFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	engine engine.Engine,
	strategy worker.ContainerPlacementStrategy,
) RadarSchedulerFactory {
	return &radarSchedulerFactory{
		conn:                         conn,
		pool:                         pool,
		resourceFactory:              resourceFactory,
		resourceTypeCheckingInterval: resourceTypeCheckingInterval,
		resourceCheckingInterval:     resourceCheckingInterval,
		engine:                       engine,
		strategy:                     strategy,
	}
}

func (rsf *radarSchedulerFactory) BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables) radar.ScanRunnerFactory {
	return radar.NewScanRunnerFactory(rsf.conn, rsf.pool, rsf.resourceFactory, rsf.resourceTypeCheckingInterval, rsf.resourceCheckingInterval, dbPipeline, clock.NewClock(), externalURL, variables, rsf.strategy)
}

func (rsf *radarSchedulerFactory) BuildScheduler(pipeline db.Pipeline, externalURL string, variables creds.Variables) scheduler.BuildScheduler {

	scanner := radar.NewResourceScanner(
		rsf.conn,
		clock.NewClock(),
		rsf.pool,
		rsf.resourceFactory,
		rsf.resourceCheckingInterval,
		pipeline,
		externalURL,
		variables,
		rsf.strategy,
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
