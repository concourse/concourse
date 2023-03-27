package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/metric"
)

type buildCollector struct {
	buildFactory buildFactory
}

type buildFactory interface {
	MarkNonInterceptibleBuilds() error
}

func NewBuildCollector(buildFactory buildFactory) *buildCollector {
	return &buildCollector{
		buildFactory: buildFactory,
	}
}

func (b *buildCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("build-collector")

	start := time.Now()
	defer func() {
		metric.BuildCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	logger.Debug("start")
	defer logger.Debug("done")

	return b.buildFactory.MarkNonInterceptibleBuilds()
}
