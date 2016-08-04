package radar

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type ResourceNotConfiguredError struct {
	ResourceName string
}

func (err ResourceNotConfiguredError) Error() string {
	return fmt.Sprintf("resource '%s' was not found in config", err.ResourceName)
}

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
	tracker resource.Tracker,
	defaultInterval time.Duration,
	db RadarDB,
	clock clock.Clock,
	externalURL string,
) ScanRunnerFactory {
	resourceScanner := NewResourceScanner(
		clock,
		tracker,
		defaultInterval,
		db,
		externalURL,
	)
	resourceTypeScanner := NewResourceTypeScanner(
		tracker,
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
