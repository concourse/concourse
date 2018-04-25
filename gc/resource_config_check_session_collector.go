package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db"
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

	err := rccsc.configCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessions()
	if err != nil {
		logger.Error("unable-to-clean-up-expired-resource-config-check-sessions", err)
		panic("XXX: dont skip")
		return err
	}

	err = rccsc.configCheckSessionLifecycle.CleanInactiveResourceConfigCheckSessions()
	if err != nil {
		logger.Error("unable-to-clean-up-resource-config-check-sessions-for-paused-and-inactive-resources", err)
		return err
	}

	return nil
}
