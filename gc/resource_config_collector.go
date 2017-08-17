package gc

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

type resourceConfigCollector struct {
	logger        lager.Logger
	configFactory db.ResourceConfigFactory
}

func NewResourceConfigCollector(
	logger lager.Logger,
	configFactory db.ResourceConfigFactory,
) Collector {
	return &resourceConfigCollector{
		logger:        logger.Session("resource-config-collector"),
		configFactory: configFactory,
	}
}

func (rcuc *resourceConfigCollector) Run() error {
	return rcuc.configFactory.CleanUnreferencedConfigs()
}
