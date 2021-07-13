package gc

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"github.com/concourse/concourse/atc/db"
)

type checksCollector struct {
	lifecycle db.CheckLifecycle
}

func NewChecksCollector(lifecycle db.CheckLifecycle) *checksCollector {
	return &checksCollector{
		lifecycle: lifecycle,
	}
}

func (c *checksCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("check-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	err := c.lifecycle.DeleteCompletedChecks(logger)
	if err != nil {
		logger.Error("failed-to-delete-completed-checks", err)
		return err
	}

	return nil
}
