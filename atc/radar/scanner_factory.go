package radar

import (
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
	NewResourceScanner(dbPipeline db.Pipeline) Scanner
	NewResourceTypeScanner(dbPipeline db.Pipeline) Scanner
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

func (f *scannerFactory) NewResourceScanner(dbPipeline db.Pipeline) Scanner {
	variables := creds.NewVariables(f.secretManager, dbPipeline.TeamName(), dbPipeline.Name())

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

func (f *scannerFactory) NewResourceTypeScanner(dbPipeline db.Pipeline) Scanner {
	variables := creds.NewVariables(f.secretManager, dbPipeline.TeamName(), dbPipeline.Name())

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
