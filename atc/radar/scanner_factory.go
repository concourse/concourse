package radar

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
)

// ScannerFactory is the same interface as resourceserver/server.go
// They are in two places because there would be cyclic dependencies otherwise

// go:generate counterfeiter . ScannerFactory
type ScannerFactory interface {
	NewResourceScanner(dbPipeline db.Pipeline) Scanner
	NewResourceTypeScanner(dbPipeline db.Pipeline) Scanner
}

type scannerFactory struct {
	resourceFactory              resource.ResourceFactory
	resourceConfigFactory        db.ResourceConfigFactory
	resourceTypeCheckingInterval time.Duration
	resourceCheckingInterval     time.Duration
	externalURL                  string
	variablesFactory             creds.VariablesFactory
	containerExpiries            db.ContainerOwnerExpiries
}

func NewScannerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	resourceTypeCheckingInterval time.Duration,
	resourceCheckingInterval time.Duration,
	externalURL string,
	variablesFactory creds.VariablesFactory,
	checkDuration time.Duration,
) ScannerFactory {
	return &scannerFactory{
		resourceFactory:              resourceFactory,
		resourceConfigFactory:        resourceConfigFactory,
		resourceCheckingInterval:     resourceCheckingInterval,
		resourceTypeCheckingInterval: resourceTypeCheckingInterval,
		externalURL:                  externalURL,
		variablesFactory:             variablesFactory,
		containerExpiries:            db.NewContainerExpiries(checkDuration),
	}
}

func (f *scannerFactory) NewResourceScanner(dbPipeline db.Pipeline) Scanner {
	variables := f.variablesFactory.NewVariables(dbPipeline.TeamName(), dbPipeline.Name())

	resourceTypeScanner := NewResourceTypeScanner(
		clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigFactory,
		f.resourceTypeCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
		f.containerExpiries,
	)

	return NewResourceScanner(
		clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigFactory,
		f.resourceCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
		resourceTypeScanner,
		f.containerExpiries,
	)
}

func (f *scannerFactory) NewResourceTypeScanner(dbPipeline db.Pipeline) Scanner {
	variables := f.variablesFactory.NewVariables(dbPipeline.TeamName(), dbPipeline.Name())

	return NewResourceTypeScanner(
		clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigFactory,
		f.resourceTypeCheckingInterval,
		dbPipeline,
		f.externalURL,
		variables,
		f.containerExpiries,
	)
}
