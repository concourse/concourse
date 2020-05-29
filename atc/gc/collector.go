package gc

import (
	"code.cloudfoundry.org/lager"
)

type Collector interface {
	Collect(lager.Logger) error
}
