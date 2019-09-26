package radar

import (
	"github.com/concourse/concourse/atc/creds"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
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
	resourceConfigFactory db.ResourceConfigFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	dbPipeline db.Pipeline,
	clock clock.Clock,
	externalURL string,
	secrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
	strategy worker.ContainerPlacementStrategy,
	notifications Notifications,
) ScanRunnerFactory {
	resourceTypeScanner := NewResourceTypeScanner(
		clock,
		pool,
		resourceConfigFactory,
		resourceTypeCheckingInterval,
		dbPipeline,
		externalURL,
		secrets,
		varSourcePool,
		strategy,
	)

	resourceScanner := NewResourceScanner(
		clock,
		pool,
		resourceConfigFactory,
		resourceCheckingInterval,
		dbPipeline,
		externalURL,
		secrets,
		varSourcePool,
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
