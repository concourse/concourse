package radar

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . Scanner

type Scanner interface {
	Run(lager.Logger, int) (time.Duration, error)
	Scan(lager.Logger, int) error
	ScanFromVersion(lager.Logger, int, atc.Version) error
}
