package pipelines

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
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
	BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables, notifications radar.Notifications) radar.ScanRunnerFactory
	BuildScheduler(pipeline db.Pipeline) scheduler.BuildScheduler
}

type radarSchedulerFactory struct {
	pool                         worker.Pool
	resourceFactory              resource.ResourceFactory
	resourceConfigFactory        db.ResourceConfigFactory
	resourceTypeCheckingInterval time.Duration
	resourceCheckingInterval     time.Duration
	strategy                     worker.ContainerPlacementStrategy
}

func NewRadarSchedulerFactory(
	pool worker.Pool,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	strategy worker.ContainerPlacementStrategy,
) RadarSchedulerFactory {
	return &radarSchedulerFactory{
		pool:                         pool,
		resourceFactory:              resourceFactory,
		resourceConfigFactory:        resourceConfigFactory,
		resourceTypeCheckingInterval: resourceTypeCheckingInterval,
		resourceCheckingInterval:     resourceCheckingInterval,
		strategy:                     strategy,
	}
}

func (rsf *radarSchedulerFactory) BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables, notifications radar.Notifications) radar.ScanRunnerFactory {
	return radar.NewScanRunnerFactory(
		rsf.pool,
		rsf.resourceFactory,
		rsf.resourceConfigFactory,
		rsf.resourceTypeCheckingInterval,
		rsf.resourceCheckingInterval,
		dbPipeline,
		clock.NewClock(),
		externalURL,
		variables,
		rsf.strategy,
		notifications,
	)
}

func (rsf *radarSchedulerFactory) BuildScheduler(pipeline db.Pipeline) scheduler.BuildScheduler {
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
			inputMapper,
		),
	}
}
