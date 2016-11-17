package gcng

//go:generate counterfeiter . Collector

type Collector interface {
	Run() error
}

type aggregateCollector struct {
	workerCollector            Collector
	resourceCacheUseCollector  Collector
	resourceConfigUseCollector Collector
	resourceConfigCollector    Collector
	resourceCacheCollector     Collector
	volumeCollector            Collector
}

func NewCollector(
	workers Collector,
	resourceCacheUses Collector,
	resourceConfigUses Collector,
	resourceConfigs Collector,
	resourceCaches Collector,
	volumes Collector,
) Collector {
	return &aggregateCollector{
		workerCollector:            workers,
		resourceCacheUseCollector:  resourceCacheUses,
		resourceConfigUseCollector: resourceConfigUses,
		resourceConfigCollector:    resourceConfigs,
		resourceCacheCollector:     resourceCaches,
		volumeCollector:            volumes,
	}
}

func (c *aggregateCollector) Run() error {
	var err error

	err = c.workerCollector.Run()
	if err != nil {
		return err
	}

	err = c.resourceCacheUseCollector.Run()
	if err != nil {
		return err
	}

	err = c.resourceConfigUseCollector.Run()
	if err != nil {
		return err
	}

	err = c.resourceConfigCollector.Run()
	if err != nil {
		return err
	}

	err = c.resourceCacheCollector.Run()
	if err != nil {
		return err
	}

	err = c.volumeCollector.Run()
	if err != nil {
		return err
	}

	return nil
}
