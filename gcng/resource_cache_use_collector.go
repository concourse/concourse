package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type ResourceCacheUseCollector interface {
	Run() error
}

type resourceCacheUseCollector struct {
	logger       lager.Logger
	cacheFactory dbng.ResourceCacheFactory
}

func NewResourceCacheUseCollector(
	logger lager.Logger,
	cacheFactory dbng.ResourceCacheFactory,
) ResourceCacheUseCollector {
	return &resourceCacheUseCollector{
		logger:       logger.Session("resource-cache-use-collector"),
		cacheFactory: cacheFactory,
	}
}

func (rcuc *resourceCacheUseCollector) Run() error {
	err := rcuc.cleanUses()
	if err != nil {
		return err
	}

	return nil
}

func (rcuc *resourceCacheUseCollector) cleanUses() error {
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
