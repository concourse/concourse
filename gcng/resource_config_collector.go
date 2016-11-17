package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type resourceConfigCollector struct {
	logger        lager.Logger
	configFactory dbng.ResourceConfigFactory
}

func NewResourceConfigCollector(
	logger lager.Logger,
	configFactory dbng.ResourceConfigFactory,
) Collector {
	return &resourceConfigCollector{
		logger:        logger.Session("resource-config-collector"),
		configFactory: configFactory,
	}
}

func (rcuc *resourceConfigCollector) Run() error {
	return rcuc.configFactory.CleanUselessConfigs()
}
