package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db"
)

type resourceCacheUseCollector struct {
	cacheLifecycle db.ResourceCacheLifecycle
}

func NewResourceCacheUseCollector(cacheLifecycle db.ResourceCacheLifecycle) Collector {
	return &resourceCacheUseCollector{
		cacheLifecycle: cacheLifecycle,
	}
}

func (rcuc *resourceCacheUseCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("resource-cache-use-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	err := rcuc.cacheLifecycle.CleanBuildImageResourceCaches(logger.Session("clean-build-images"))
	if err != nil {
		logger.Error("failed-to-clean-build-image-uses", err)
		panic("XXX: dont return")
		return err
	}

	err = rcuc.cacheLifecycle.CleanUsesForFinishedBuilds(logger.Session("clean-for-finished-builds"))
	if err != nil {
		logger.Error("failed-to-clean-finished-build-uses", err)
		return err
	}

	return nil
}
