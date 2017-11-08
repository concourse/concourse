package radar

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Scanner

type Scanner interface {
	Run(lager.Logger, string) (time.Duration, error)
	Scan(lager.Logger, string) error
	ScanFromVersion(lager.Logger, string, atc.Version) error
}

//go:generate counterfeiter . ScanRunnerFactory

type ScanRunnerFactory interface {
	ScanResourceRunner(lager.Logger, string) IntervalRunner
	ScanResourceTypeRunner(lager.Logger, string) IntervalRunner
}

type scanRunnerFactory struct {
	clock               clock.Clock
	resourceScanner     Scanner
	resourceTypeScanner Scanner
}

func NewScanRunnerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	clock clock.Clock,
	externalURL string,
	variables creds.Variables,
) ScanRunnerFactory {
	resourceTypeScanner := NewResourceTypeScanner(
		clock,
		resourceFactory,
		resourceConfigCheckSessionFactory,
		defaultInterval,
		dbPipeline,
		externalURL,
		variables,
	)

	resourceScanner := NewResourceScanner(
		clock,
		resourceFactory,
		resourceConfigCheckSessionFactory,
		defaultInterval,
		dbPipeline,
		externalURL,
		variables,
		resourceTypeScanner,
	)
	return &scanRunnerFactory{
		clock:               clock,
		resourceScanner:     resourceScanner,
		resourceTypeScanner: resourceTypeScanner,
	}
}

func (sf *scanRunnerFactory) ScanResourceRunner(logger lager.Logger, name string) IntervalRunner {
	return NewIntervalRunner(logger.Session("interval-runner"), sf.clock, name, sf.resourceScanner)
}

func (sf *scanRunnerFactory) ScanResourceTypeRunner(logger lager.Logger, name string) IntervalRunner {
	return NewIntervalRunner(logger.Session("interval-runner"), sf.clock, name, sf.resourceTypeScanner)
}
