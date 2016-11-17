package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type resourceCacheCollector struct {
	logger        lager.Logger
	cacheFactory  dbng.ResourceCacheFactory
	configFactory dbng.ResourceConfigFactory
}

func NewResourceCacheCollector(
	logger lager.Logger,
	cacheFactory dbng.ResourceCacheFactory,
	configFactory dbng.ResourceConfigFactory,
) Collector {
	return &resourceCacheCollector{
		logger:        logger.Session("resource-cache-use-collector"),
		cacheFactory:  cacheFactory,
		configFactory: configFactory,
	}
}

func (rcuc *resourceCacheCollector) Run() error {
	err := rcuc.cleanConfigUses()
	if err != nil {
		return err
	}

	err = rcuc.cleanCacheUses()
	if err != nil {
		return err
	}

	err = rcuc.cleanConfigs()
	if err != nil {
		return err
	}

	err = rcuc.cleanCaches()
	if err != nil {
		return err
	}

	return nil
}

func (rcuc *resourceCacheCollector) cleanConfigUses() error {
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
		rcuc.logger.Error("unable-to-clean-up-for-resources", err)
		return err
	}

	return nil
}

func (rcuc *resourceCacheCollector) cleanConfigs() error {
	err := rcuc.configFactory.CleanUselessConfigs()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-configs", err)
		return err
	}

	return nil
}

func (rcuc *resourceCacheCollector) cleanCacheUses() error {
	err := rcuc.cacheFactory.CleanUsesForFinishedBuilds()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-for-builds", err)
		return err
	}

	err = rcuc.cacheFactory.CleanUsesForInactiveResourceTypes()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-for-types", err)
		return err
	}

	err = rcuc.cacheFactory.CleanUsesForInactiveResources()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-for-resources", err)
		return err
	}

	return nil
}

func (rcuc *resourceCacheCollector) cleanCaches() error {
	return rcuc.cacheFactory.CleanUpInvalidCaches()
}
