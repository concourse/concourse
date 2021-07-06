package gc

import (
	"context"
	"github.com/concourse/concourse/atc/component"
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

func (rcc *resourceCacheCollector) Run(ctx context.Context, _ string) (component.RunResult, error) {
	logger := lagerctx.FromContext(ctx).Session("resource-cache-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.ResourceCacheCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	return nil, rcc.cacheLifecycle.CleanUpInvalidCaches(logger)
}
