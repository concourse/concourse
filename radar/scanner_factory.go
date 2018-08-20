package radar

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
)

// ScannerFactory is the same interface as resourceserver/server.go
// They are in two places because there would be cyclic dependencies otherwise

// go:generate counterfeiter . ScannerFactory
type ScannerFactory interface {
	NewResourceScanner(dbPipeline db.Pipeline) Scanner
	NewResourceTypeScanner(dbPipeline db.Pipeline) Scanner
}

type scannerFactory struct {
	resourceFactory                   resource.ResourceFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	resourceTypeCheckingInterval      time.Duration
	resourceCheckingInterval          time.Duration
	externalURL                       string
	variablesFactory                  creds.VariablesFactory
}

var ContainerExpiries = db.ContainerOwnerExpiries{
	GraceTime: 2 * time.Minute,
	Min:       5 * time.Minute,
	Max:       1 * time.Hour,
}

func NewScannerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	externalURL string,
	variablesFactory creds.VariablesFactory,
) ScannerFactory {
	return &scannerFactory{
		resourceFactory:                   resourceFactory,
		resourceConfigCheckSessionFactory: resourceConfigCheckSessionFactory,
		resourceCheckingInterval:          resourceCheckingInterval,
		resourceTypeCheckingInterval:      resourceTypeCheckingInterval,
		externalURL:                       externalURL,
		variablesFactory:                  variablesFactory,
	}
}

func (f *scannerFactory) NewResourceScanner(dbPipeline db.Pipeline) Scanner {
	variables := f.variablesFactory.NewVariables(dbPipeline.TeamName(), dbPipeline.Name())

	resourceTypeScanner := NewResourceTypeScanner(
		clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigCheckSessionFactory,
		f.resourceTypeCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
	)

	return NewResourceScanner(
		clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigCheckSessionFactory,
		f.resourceCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
		resourceTypeScanner,
	)
}

func (f *scannerFactory) NewResourceTypeScanner(dbPipeline db.Pipeline) Scanner {
	variables := f.variablesFactory.NewVariables(dbPipeline.TeamName(), dbPipeline.Name())

	return NewResourceTypeScanner(
		clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigCheckSessionFactory,
		f.resourceTypeCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
	)
}
