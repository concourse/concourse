package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type resourceConfigUseCollector struct {
	logger        lager.Logger
	configFactory dbng.ResourceConfigFactory
}

func NewResourceConfigUseCollector(
	logger lager.Logger,
	configFactory dbng.ResourceConfigFactory,
) Collector {
	return &resourceConfigUseCollector{
		logger:        logger.Session("resource-cache-use-collector"),
		configFactory: configFactory,
	}
}

func (rcuc *resourceConfigUseCollector) Run() error {
	err := rcuc.configFactory.CleanConfigUsesForFinishedBuilds()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-config-uses", err)
		return err
	}

	err = rcuc.configFactory.CleanConfigUsesForInactiveResourceTypes()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-for-types", err)
		return err
	}

	err = rcuc.configFactory.CleanConfigUsesForInactiveResources()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-for-inactive-resources", err)
		return err
	}

	err = rcuc.configFactory.CleanConfigUsesForPausedPipelinesResources()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-for-paused-resources", err)
		return err
	}

	return nil
}
