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
	resourceFactory resource.ResourceFactory
	defaultInterval time.Duration
	externalURL     string
}

func NewScannerFactory(
	resourceFactory resource.ResourceFactory,
	defaultInterval time.Duration,
	externalURL string,
) ScannerFactory {
	return &scannerFactory{
		resourceFactory: resourceFactory,
		defaultInterval: defaultInterval,
		externalURL:     externalURL,
	}
}

func (f *scannerFactory) NewResourceScanner(dbPipeline db.Pipeline) Scanner {
	return NewResourceScanner(clock.NewClock(), f.resourceFactory, f.defaultInterval, dbPipeline, f.externalURL)
}
