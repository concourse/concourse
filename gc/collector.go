package gc

import (
	"sync"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Collector

type Collector interface {
	Run() error
}

type aggregateCollector struct {
	logger                              lager.Logger
	buildCollector                      Collector
	workerCollector                     Collector
	resourceCacheUseCollector           Collector
	resourceConfigUseCollector          Collector
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
	resourceConfigUses Collector,
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
		resourceConfigUseCollector:          resourceConfigUses,
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

	err = c.resourceConfigUseCollector.Run()
	if err != nil {
		c.logger.Error("failed-to-run-resource-config-use-collector", err)
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

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := c.containerCollector.Run()
		if err != nil {
			c.logger.Error("container-collector", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := c.volumeCollector.Run()
		if err != nil {
			c.logger.Error("volume-collector", err)
		}
	}()

	wg.Wait()

	return nil
}
