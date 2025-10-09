package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

type oldBuildCollector struct {
	buildFactory db.BuildFactory
}

func NewOldBuildCollector(buildFactory db.BuildFactory) *oldBuildCollector {
	return &oldBuildCollector{
		buildFactory: buildFactory,
	}
}

func (b *oldBuildCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("old-build-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.OldBuildCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	return b.buildFactory.CleanupOldBuilds()
}