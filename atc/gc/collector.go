package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
)

//go:generate counterfeiter . Collector

type Collector interface {
	Run(context.Context) error
}

type aggregateCollector struct {
	buildCollector                      Collector
	workerCollector                     Collector
	resourceCacheUseCollector           Collector
	resourceConfigCollector             Collector
	resourceCacheCollector              Collector
	volumeCollector                     Collector
	containerCollector                  Collector
	resourceConfigCheckSessionCollector Collector
}

func NewCollector(
	buildCollector Collector,
	workers Collector,
	resourceCacheUses Collector,
	resourceConfigs Collector,
	resourceCaches Collector,
	volumes Collector,
	containers Collector,
	resourceConfigCheckSessionCollector Collector,
) Collector {
	return &aggregateCollector{
		buildCollector:                      buildCollector,
		workerCollector:                     workers,
		resourceCacheUseCollector:           resourceCacheUses,
		resourceConfigCollector:             resourceConfigs,
		resourceCacheCollector:              resourceCaches,
		volumeCollector:                     volumes,
		containerCollector:                  containers,
		resourceConfigCheckSessionCollector: resourceConfigCheckSessionCollector,
	}
}

func (c *aggregateCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	var err error

	err = c.buildCollector.Run(ctx)
	if err != nil {
		logger.Error("failed-to-run-build-collector", err)
	}

	err = c.workerCollector.Run(ctx)
	if err != nil {
		logger.Error("failed-to-run-worker-collector", err)
	}

	err = c.resourceCacheUseCollector.Run(ctx)
	if err != nil {
		logger.Error("failed-to-run-resource-cache-use-collector", err)
	}

	err = c.resourceConfigCollector.Run(ctx)
	if err != nil {
		logger.Error("failed-to-run-resource-config-collector", err)
	}

	err = c.resourceCacheCollector.Run(ctx)
	if err != nil {
		logger.Error("failed-to-run-resource-cache-collector", err)
	}

	err = c.resourceConfigCheckSessionCollector.Run(ctx)
	if err != nil {
		logger.Error("resource-config-check-session-collector", err)
	}

	err = c.containerCollector.Run(ctx)
	if err != nil {
		logger.Error("container-collector", err)
	}

	err = c.volumeCollector.Run(ctx)
	if err != nil {
		logger.Error("volume-collector", err)
	}

	return nil
}
