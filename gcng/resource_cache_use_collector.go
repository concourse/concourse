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
	err := rcuc.cacheFactory.CleanUsesForFinishedBuilds()
	if err != nil {
		rcuc.logger.Error("unable-to-clean-up-for-builds", err)
		return err
	}
	return nil
}
