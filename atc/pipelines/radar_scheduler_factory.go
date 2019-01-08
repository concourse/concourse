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
	resourceFactory              resource.ResourceFactory
	resourceConfigFactory        db.ResourceConfigFactory
	resourceTypeCheckingInterval time.Duration
	resourceCheckingInterval     time.Duration
	engine                       engine.Engine
	containerExpiries            db.ContainerOwnerExpiries
}

func NewRadarSchedulerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	engine engine.Engine,
	checkDuration time.Duration,
) RadarSchedulerFactory {
	return &radarSchedulerFactory{
		resourceFactory:              resourceFactory,
		resourceConfigFactory:        resourceConfigFactory,
		resourceTypeCheckingInterval: resourceTypeCheckingInterval,
		resourceCheckingInterval:     resourceCheckingInterval,
		engine:                       engine,
		containerExpiries:            db.NewContainerExpiries(checkDuration),
	}
}

func (rsf *radarSchedulerFactory) BuildScanRunnerFactory(dbPipeline db.Pipeline, externalURL string, variables creds.Variables) radar.ScanRunnerFactory {
	return radar.NewScanRunnerFactory(rsf.resourceFactory, rsf.resourceConfigFactory, rsf.resourceTypeCheckingInterval, rsf.resourceCheckingInterval, dbPipeline, clock.NewClock(), externalURL, variables, rsf.containerExpiries)
}

func (rsf *radarSchedulerFactory) BuildScheduler(pipeline db.Pipeline, externalURL string, variables creds.Variables) scheduler.BuildScheduler {

	resourceTypeScanner := radar.NewResourceTypeScanner(
		clock.NewClock(),
		rsf.resourceFactory,
		rsf.resourceConfigFactory,
		rsf.resourceTypeCheckingInterval,
		pipeline,
		externalURL,
		variables,
		rsf.containerExpiries,
	)

	scanner := radar.NewResourceScanner(
		clock.NewClock(),
		rsf.resourceFactory,
		rsf.resourceConfigFactory,
		rsf.resourceCheckingInterval,
		pipeline,
		externalURL,
		variables,
		resourceTypeScanner,
		rsf.containerExpiries,
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
