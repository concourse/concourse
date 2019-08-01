package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

type resourceConfigCollector struct {
	configFactory db.ResourceConfigFactory
}

func NewResourceConfigCollector(configFactory db.ResourceConfigFactory) *resourceConfigCollector {
	return &resourceConfigCollector{
		configFactory: configFactory,
	}
}

func (rcuc *resourceConfigCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("resource-config-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.ResourceConfigCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	return rcuc.configFactory.CleanUnreferencedConfigs()
}
