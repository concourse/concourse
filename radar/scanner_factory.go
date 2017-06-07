package radar

import (
	"time"

	"code.cloudfoundry.org/clock"
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
}

func NewScannerFactory(
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultInterval time.Duration,
	externalURL string,
) ScannerFactory {
	return &scannerFactory{
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultInterval:       defaultInterval,
		externalURL:           externalURL,
	}
}

func (f *scannerFactory) NewResourceScanner(dbPipeline db.Pipeline) Scanner {
	return NewResourceScanner(clock.NewClock(), f.resourceFactory, f.resourceConfigFactory, f.defaultInterval, dbPipeline, f.externalURL)
}
