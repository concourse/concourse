package radar

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/resource"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Scanner

type Scanner interface {
	Run(lager.Logger, string) (time.Duration, error)
	Scan(lager.Logger, string) error
	ScanFromVersion(lager.Logger, string, atc.Version) error
}

type ScanRunnerFactory interface {
	ScanResourceRunner(lager.Logger, string) ifrit.Runner
	ScanResourceTypeRunner(lager.Logger, string) ifrit.Runner
}

type scanRunnerFactory struct {
	clock               clock.Clock
	resourceScanner     Scanner
	resourceTypeScanner Scanner
}

func NewScanRunnerFactory(
	resourceFactory resource.ResourceFactory,
	defaultInterval time.Duration,
	db RadarDB,
	dbPipeline dbng.Pipeline,
	clock clock.Clock,
	externalURL string,
) ScanRunnerFactory {
	resourceScanner := NewResourceScanner(
		clock,
		resourceFactory,
		defaultInterval,
		db,
		dbPipeline,
		externalURL,
	)
	resourceTypeScanner := NewResourceTypeScanner(
		resourceFactory,
		defaultInterval,
		db,
		externalURL,
	)

	return &scanRunnerFactory{
		clock:               clock,
		resourceScanner:     resourceScanner,
		resourceTypeScanner: resourceTypeScanner,
	}
}

func (sf *scanRunnerFactory) ScanResourceRunner(logger lager.Logger, name string) ifrit.Runner {
	intervalRunner := NewIntervalRunner(logger, sf.clock, name, sf.resourceScanner)
	return ifrit.RunFunc(intervalRunner.RunFunc)
}

func (sf *scanRunnerFactory) ScanResourceTypeRunner(logger lager.Logger, name string) ifrit.Runner {
	intervalRunner := NewIntervalRunner(logger, sf.clock, name, sf.resourceTypeScanner)
	return ifrit.RunFunc(intervalRunner.RunFunc)
}
