package gc

import (
	"context"
	"github.com/concourse/concourse/atc/component"

	"code.cloudfoundry.org/lager/lagerctx"
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

func (c *checksCollector) Run(ctx context.Context, _ string) (component.RunResult, error) {
	logger := lagerctx.FromContext(ctx).Session("check-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	err := c.lifecycle.DeleteCompletedChecks(logger)
	if err != nil {
		logger.Error("failed-to-delete-completed-checks", err)
		return nil, err
	}

	return nil, nil
}
