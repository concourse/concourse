package radar

import (
	"fmt"
	"time"

	"github.com/concourse/atc/resource"
	"github.com/tedsuo/ifrit"

	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
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
}

type ScannerFactory interface {
	ScanResourceRunner(lager.Logger, string) ifrit.Runner
	ScanResourceTypeRunner(lager.Logger, string) ifrit.Runner
}

type scannerFactory struct {
	clock               clock.Clock
	resourceScanner     Scanner
	resourceTypeScanner Scanner
}

func NewScannerFactory(
	tracker resource.Tracker,
	defaultInterval time.Duration,
	db RadarDB,
	clock clock.Clock,
	externalURL string,
) ScannerFactory {
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

	return &scannerFactory{
		clock:               clock,
		resourceScanner:     resourceScanner,
		resourceTypeScanner: resourceTypeScanner,
	}
}

func (sf *scannerFactory) ScanResourceRunner(logger lager.Logger, name string) ifrit.Runner {
	intervalRunner := NewIntervalRunner(logger, sf.clock, name, sf.resourceScanner)
	return ifrit.RunFunc(intervalRunner.RunFunc)
}

func (sf *scannerFactory) ScanResourceTypeRunner(logger lager.Logger, name string) ifrit.Runner {
	intervalRunner := NewIntervalRunner(logger, sf.clock, name, sf.resourceTypeScanner)
	return ifrit.RunFunc(intervalRunner.RunFunc)
}
