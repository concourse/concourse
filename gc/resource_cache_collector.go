package gc

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

type resourceCacheCollector struct {
	logger         lager.Logger
	cacheLifecycle db.ResourceCacheLifecycle
}

func NewResourceCacheCollector(
	logger lager.Logger,
	cacheLifecycle db.ResourceCacheLifecycle,
) Collector {
	return &resourceCacheCollector{
		logger:         logger.Session("resource-cache-collector"),
		cacheLifecycle: cacheLifecycle,
	}
}

func (rcc *resourceCacheCollector) Run() error {
	return rcc.cacheLifecycle.CleanUpInvalidCaches(rcc.logger)
}
