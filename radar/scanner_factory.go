package radar

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/atc/resource"
)

type ScannerFactory interface {
	NewResourceScanner(db RadarDB) Scanner
}

type scannerFactory struct {
	tracker         resource.Tracker
	defaultInterval time.Duration
	externalURL     string
}

func NewScannerFactory(
	tracker resource.Tracker,
	defaultInterval time.Duration,
	externalURL string,
) ScannerFactory {
	return &scannerFactory{
		tracker:         tracker,
		defaultInterval: defaultInterval,
		externalURL:     externalURL,
	}
}

func (f *scannerFactory) NewResourceScanner(db RadarDB) Scanner {
	return NewResourceScanner(clock.NewClock(), f.tracker, f.defaultInterval, db, f.externalURL)
}
