package gcng

//go:generate counterfeiter . Collector

type Collector interface {
	Run() error
}

type aggregateCollector struct {
	workerCollector        Collector
	resourceCacheCollector Collector
	volumeCollector        Collector
}

func NewCollector(
	workers Collector,
	resourceCaches Collector,
	volumes Collector,
) Collector {
	return &aggregateCollector{
		workerCollector:        workers,
		resourceCacheCollector: resourceCaches,
		volumeCollector:        volumes,
	}
}

func (c *aggregateCollector) Run() error {
	var err error

	err = c.workerCollector.Run()
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
