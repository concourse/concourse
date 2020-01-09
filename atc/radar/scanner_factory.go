package radar

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
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
	resourceConfigFactory        db.ResourceConfigFactory
	resourceTypeCheckingInterval time.Duration
	resourceCheckingInterval     time.Duration
	externalURL                  string
	secretManager                creds.Secrets
	varSourcePool                creds.VarSourcePool
	strategy                     worker.ContainerPlacementStrategy
}

var ContainerExpiries = db.ContainerOwnerExpiries{
	Min: 5 * time.Minute,
	Max: 1 * time.Hour,
}

func NewScannerFactory(
	pool worker.Pool,
	resourceConfigFactory db.ResourceConfigFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	externalURL string,
	secretManager creds.Secrets,
	varSourcePool creds.VarSourcePool,
	strategy worker.ContainerPlacementStrategy,
) ScannerFactory {
	return &scannerFactory{
		pool:                         pool,
		resourceConfigFactory:        resourceConfigFactory,
		resourceCheckingInterval:     resourceCheckingInterval,
		resourceTypeCheckingInterval: resourceTypeCheckingInterval,
		externalURL:                  externalURL,
		secretManager:                secretManager,
		varSourcePool:                varSourcePool,
		strategy:                     strategy,
	}
}

func (f *scannerFactory) NewResourceScanner(logger lager.Logger, dbPipeline db.Pipeline) Scanner {
	return NewResourceScanner(
		clock.NewClock(),
		f.pool,
		resource.NewResourceFactory(),
		f.resourceConfigFactory,
		f.resourceCheckingInterval,
		dbPipeline,
		f.externalURL,
		f.secretManager,
		f.varSourcePool,
		f.strategy,
	)
}

func (f *scannerFactory) NewResourceTypeScanner(logger lager.Logger, dbPipeline db.Pipeline) Scanner {
	return NewResourceTypeScanner(
		clock.NewClock(),
		f.pool,
		resource.NewResourceFactory(),
		f.resourceConfigFactory,
		f.resourceTypeCheckingInterval,
		dbPipeline,
		f.externalURL,
		f.secretManager,
		f.varSourcePool,
		f.strategy,
	)
}
