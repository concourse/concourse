package radar

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
)

type ScannerFactory interface {
	NewResourceScanner(dbPipeline db.Pipeline) Scanner
}

type scannerFactory struct {
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	defaultInterval       time.Duration
	externalURL           string
	variablesFactory      creds.VariablesFactory
}

func NewScannerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultInterval time.Duration,
	externalURL string,
	variablesFactory creds.VariablesFactory,
) ScannerFactory {
	return &scannerFactory{
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultInterval:       defaultInterval,
		externalURL:           externalURL,
		variablesFactory:      variablesFactory,
	}
}

func (f *scannerFactory) NewResourceScanner(dbPipeline db.Pipeline) Scanner {
	return NewResourceScanner(clock.NewClock(), f.resourceFactory, f.resourceConfigFactory, f.defaultInterval, dbPipeline, f.externalURL, f.variablesFactory.NewVariables(dbPipeline.TeamName(), dbPipeline.Name()))
}
