package gc

import (
	"code.cloudfoundry.org/lager/v3"
)

type Collector interface {
	Collect(lager.Logger) error
}
