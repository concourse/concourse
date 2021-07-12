package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

type taskCacheCollector struct {
	cacheLifecycle db.TaskCacheLifecycle
}

func NewTaskCacheCollector(cacheLifecycle db.TaskCacheLifecycle) *taskCacheCollector {
	return &taskCacheCollector{
		cacheLifecycle: cacheLifecycle,
	}
}

func (rcc *taskCacheCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("task-cache-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.TaskCacheCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	deletedCacheIDs, err := rcc.cacheLifecycle.CleanUpInvalidTaskCaches()
	if err != nil {
		return err
	}

	if len(deletedCacheIDs) > 0 {
		logger.Debug("deleted-task-caches", lager.Data{"id": deletedCacheIDs})
	}

	return nil
}
