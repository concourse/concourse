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
	resourceFactory                   resource.ResourceFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	defaultInterval                   time.Duration
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
	defaultInterval time.Duration,
	externalURL string,
	variablesFactory creds.VariablesFactory,
) ScannerFactory {
	return &scannerFactory{
		resourceFactory:                   resourceFactory,
		resourceConfigCheckSessionFactory: resourceConfigCheckSessionFactory,
		defaultInterval:                   defaultInterval,
		externalURL:                       externalURL,
		variablesFactory:                  variablesFactory,
	}
}

func (f *scannerFactory) NewResourceScanner(dbPipeline db.Pipeline) Scanner {
	resourceTypeScanner := NewResourceTypeScanner(
		clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigCheckSessionFactory,
		f.defaultInterval,
		dbPipeline,
		f.externalURL,
		f.variablesFactory.NewVariables(dbPipeline.TeamName(), dbPipeline.Name()),
	)

	return NewResourceScanner(clock.NewClock(),
		f.resourceFactory,
		f.resourceConfigCheckSessionFactory,
		f.defaultInterval,
		dbPipeline,
		f.externalURL,
		f.variablesFactory.NewVariables(dbPipeline.TeamName(), dbPipeline.Name()),
		resourceTypeScanner,
	)
}
