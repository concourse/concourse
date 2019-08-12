package radar

import (
	"time"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ScanRunnerFactory

type ScanRunnerFactory interface {
	ScanResourceRunner(lager.Logger, db.Resource) IntervalRunner
	ScanResourceTypeRunner(lager.Logger, db.ResourceType) IntervalRunner
}

type scanRunnerFactory struct {
	clock               clock.Clock
	resourceScanner     Scanner
	resourceTypeScanner Scanner
	notifications       Notifications
}

func NewScanRunnerFactory(
	pool worker.Pool,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	dbPipeline db.Pipeline,
	clock clock.Clock,
	externalURL string,
	variables vars.Variables,
	strategy worker.ContainerPlacementStrategy,
	notifications Notifications,
) ScanRunnerFactory {
	resourceTypeScanner := NewResourceTypeScanner(
		clock,
		pool,
		resourceFactory,
		resourceConfigFactory,
		resourceTypeCheckingInterval,
		dbPipeline,
		externalURL,
		variables,
		strategy,
	)

	resourceScanner := NewResourceScanner(
		clock,
		pool,
		resourceFactory,
		resourceConfigFactory,
		resourceCheckingInterval,
		dbPipeline,
		externalURL,
		variables,
		strategy,
	)
	return &scanRunnerFactory{
		clock:               clock,
		resourceScanner:     resourceScanner,
		resourceTypeScanner: resourceTypeScanner,
		notifications:       notifications,
	}
}

func (sf *scanRunnerFactory) ScanResourceRunner(logger lager.Logger, resource db.Resource) IntervalRunner {
	return NewIntervalRunner(
		logger.Session("interval-runner"),
		sf.clock,
		resource.ID(),
		sf.resourceScanner,
		sf.notifications,
	)
}

func (sf *scanRunnerFactory) ScanResourceTypeRunner(logger lager.Logger, resourceType db.ResourceType) IntervalRunner {
	return NewIntervalRunner(
		logger.Session("interval-runner"),
		sf.clock,
		resourceType.ID(),
		sf.resourceTypeScanner,
		sf.notifications,
	)
}
