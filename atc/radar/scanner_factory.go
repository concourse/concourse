package radar

import (
	"code.cloudfoundry.org/lager"
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

// ScannerFactory is the same interface as resourceserver/server.go
// They are in two places because there would be cyclic dependencies otherwise

// go:generate counterfeiter . ScannerFactory
type ScannerFactory interface {
	NewResourceScanner(lager.Logger, db.Pipeline) Scanner
	NewResourceTypeScanner(lager.Logger, db.Pipeline) Scanner
}

type scannerFactory struct {
	pool                         worker.Pool
	resourceFactory              resource.ResourceFactory
	resourceConfigFactory        db.ResourceConfigFactory
	resourceTypeCheckingInterval time.Duration
	resourceCheckingInterval     time.Duration
	externalURL                  string
	secretManager                creds.Secrets
	strategy                     worker.ContainerPlacementStrategy
}

var ContainerExpiries = db.ContainerOwnerExpiries{
	Min: 5 * time.Minute,
	Max: 1 * time.Hour,
}

func NewScannerFactory(
	pool worker.Pool,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	externalURL string,
	secretManager creds.Secrets,
	strategy worker.ContainerPlacementStrategy,
) ScannerFactory {
	return &scannerFactory{
		pool:                         pool,
		resourceFactory:              resourceFactory,
		resourceConfigFactory:        resourceConfigFactory,
		resourceCheckingInterval:     resourceCheckingInterval,
		resourceTypeCheckingInterval: resourceTypeCheckingInterval,
		externalURL:                  externalURL,
		secretManager:                secretManager,
		strategy:                     strategy,
	}
}

func (f *scannerFactory) NewResourceScanner(logger lager.Logger, dbPipeline db.Pipeline) Scanner {
	globalVariables := creds.NewVariables(f.secretManager, dbPipeline.TeamName(), dbPipeline.Name(), false)
	variables, err := dbPipeline.Variables(logger, globalVariables)
	if err != nil {
		return nil
	}

	return NewResourceScanner(
		clock.NewClock(),
		f.pool,
		f.resourceFactory,
		f.resourceConfigFactory,
		f.resourceCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
		f.strategy,
	)
}

func (f *scannerFactory) NewResourceTypeScanner(logger lager.Logger, dbPipeline db.Pipeline) Scanner {
	globalVariables := creds.NewVariables(f.secretManager, dbPipeline.TeamName(), dbPipeline.Name(), false)
	variables, err := dbPipeline.Variables(logger, globalVariables)
	if err != nil {
		return nil
	}

	return NewResourceTypeScanner(
		clock.NewClock(),
		f.pool,
		f.resourceFactory,
		f.resourceConfigFactory,
		f.resourceTypeCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
		f.strategy,
	)
}
