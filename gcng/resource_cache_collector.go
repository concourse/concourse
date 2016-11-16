package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type ResourceCacheCollector interface {
	Run() error
}

type resourceCacheCollector struct {
	logger       lager.Logger
	cacheFactory dbng.ResourceCacheFactory
}

func NewResourceCacheCollector(
	logger lager.Logger,
	cacheFactory dbng.ResourceCacheFactory,
) ResourceCacheCollector {
	return &resourceCacheCollector{
		logger:       logger.Session("resource-cache-use-collector"),
		cacheFactory: cacheFactory,
	}
}

func (rcuc *resourceCacheCollector) Run() error {
	err := rcuc.cleanUses()
	if err != nil {
		return err
	}

	err = rcuc.cleanCaches()
	if err != nil {
		return err
	}

	return nil
}

func (rcuc *resourceCacheCollector) cleanUses() error {
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
