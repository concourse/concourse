package gc

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

type resourceCacheUseCollector struct {
	logger         lager.Logger
	cacheLifecycle db.ResourceCacheLifecycle
}

func NewResourceCacheUseCollector(
	logger lager.Logger,
	cacheLifecycle db.ResourceCacheLifecycle,
) Collector {
	return &resourceCacheUseCollector{
		logger:         logger.Session("resource-cache-use-collector"),
		cacheLifecycle: cacheLifecycle,
	}
}

func (rcuc *resourceCacheUseCollector) Run() error {
	err := rcuc.cacheLifecycle.CleanBuildImageResourceCaches(rcuc.logger.Session("clean-build-images"))
	if err != nil {
		rcuc.logger.Error("error-cleaning-build-image-uses", err)
		return err
	}

	err = rcuc.cacheLifecycle.CleanUsesForFinishedBuilds(rcuc.logger.Session("clean-for-finished-builds"))
	if err != nil {
		rcuc.logger.Error("error-cleaning-finished-build-uses", err)
		return err
	}

	return nil
}
