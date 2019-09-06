package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

type checkCollector struct {
	checkLifecycle db.CheckLifecycle
	recyclePeriod  time.Duration
}

func NewCheckCollector(checkLifecycle db.CheckLifecycle, recyclePeriod time.Duration) *checkCollector {
	return &checkCollector{
		checkLifecycle: checkLifecycle,
		recyclePeriod:  recyclePeriod,
	}
}

func (c *checkCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("check-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	return c.checkLifecycle.RemoveExpiredChecks(c.recyclePeriod)
}
