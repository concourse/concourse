package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db"
	multierror "github.com/hashicorp/go-multierror"
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

	var errs error

	err := rcuc.cacheLifecycle.CleanBuildImageResourceCaches(logger.Session("clean-build-images"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-build-image-uses", err)
	}

	err = rcuc.cacheLifecycle.CleanUsesForFinishedBuilds(logger.Session("clean-for-finished-builds"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-finished-build-uses", err)
	}

	return errs
}
