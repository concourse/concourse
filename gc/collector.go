package gc

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . Collector

type Collector interface {
	Run() error
}

type aggregateCollector struct {
	logger                              lager.Logger
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
	logger lager.Logger,
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
		logger:                              logger,
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

func (c *aggregateCollector) Run() error {
	var err error

	err = c.buildCollector.Run()
	if err != nil {
		c.logger.Error("failed-to-run-build-collector", err)
	}

	err = c.workerCollector.Run()
	if err != nil {
		c.logger.Error("failed-to-run-worker-collector", err)
	}

	err = c.resourceCacheUseCollector.Run()
	if err != nil {
		c.logger.Error("failed-to-run-resource-cache-use-collector", err)
	}

	err = c.resourceConfigCollector.Run()
	if err != nil {
		c.logger.Error("failed-to-run-resource-config-collector", err)
	}

	err = c.resourceCacheCollector.Run()
	if err != nil {
		c.logger.Error("failed-to-run-resource-cache-collector", err)
	}

	err = c.resourceConfigCheckSessionCollector.Run()
	if err != nil {
		c.logger.Error("resource-config-check-session-collector", err)
	}

	err = c.containerCollector.Run()
	if err != nil {
		c.logger.Error("container-collector", err)
	}

	err = c.volumeCollector.Run()
	if err != nil {
		c.logger.Error("volume-collector", err)
	}

	return nil
}
