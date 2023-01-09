package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

type resourceCacheCollector struct {
	cacheLifecycle db.ResourceCacheLifecycle
}

func NewResourceCacheCollector(cacheLifecycle db.ResourceCacheLifecycle) *resourceCacheCollector {
	return &resourceCacheCollector{
		cacheLifecycle: cacheLifecycle,
	}
}

func (rcc *resourceCacheCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("resource-cache-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.ResourceCacheCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	err := rcc.cacheLifecycle.CleanUpInvalidCaches(logger)
	if err != nil {
		return err
	}

	err = rcc.cacheLifecycle.CleanInvalidWorkerResourceCaches(logger, 500)
	if err != nil {
		return err
	}

	return nil
}
