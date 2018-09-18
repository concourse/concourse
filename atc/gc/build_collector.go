package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
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

	logger.Debug("start")
	defer logger.Debug("done")

	return b.buildFactory.MarkNonInterceptibleBuilds()
}
