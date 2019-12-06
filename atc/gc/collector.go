package gc

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/lockrunner"
)

type Collector interface {
	Collect(lager.Logger) error
}

func NewCollectorTask(collector Collector) lockrunner.Task {
	return &collectorTask{collector: collector}
}

type collectorTask struct {
	collector Collector
}

func (c *collectorTask) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("tick")

	logger.Debug("start")
	defer logger.Debug("done")

	return c.collector.Collect(logger)
}
