package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db"
	multierror "github.com/hashicorp/go-multierror"
)

type resourceConfigCheckSessionCollector struct {
	configCheckSessionLifecycle db.ResourceConfigCheckSessionLifecycle
}

func NewResourceConfigCheckSessionCollector(
	configCheckSessionLifecycle db.ResourceConfigCheckSessionLifecycle,
) Collector {
	return &resourceConfigCheckSessionCollector{
		configCheckSessionLifecycle: configCheckSessionLifecycle,
	}
}

func (rccsc *resourceConfigCheckSessionCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("resource-config-check-session-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	var errs error

	err := rccsc.configCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessions()
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-expired-resource-config-check-sessions", err)
	}

	err = rccsc.configCheckSessionLifecycle.CleanInactiveResourceConfigCheckSessions()
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-inactive-resource-config-check-sessions", err)
	}

	return errs
}
