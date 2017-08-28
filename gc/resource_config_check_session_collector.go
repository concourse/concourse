package gc

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

type resourceConfigCheckSessionCollector struct {
	logger                      lager.Logger
	configCheckSessionLifecycle db.ResourceConfigCheckSessionLifecycle
}

func NewResourceConfigCheckSessionCollector(
	logger lager.Logger,
	configCheckSessionLifecycle db.ResourceConfigCheckSessionLifecycle,
) Collector {
	return &resourceConfigCheckSessionCollector{
		logger: logger.Session("resource-config-check-session-collector"),
		configCheckSessionLifecycle: configCheckSessionLifecycle,
	}
}

func (rccsc *resourceConfigCheckSessionCollector) Run() error {
	err := rccsc.configCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessions()
	if err != nil {
		rccsc.logger.Error("unable-to-clean-up-expired-resource-config-check-sessions", err)
		return err
	}

	err = rccsc.configCheckSessionLifecycle.CleanInactiveResourceConfigCheckSessions()
	if err != nil {
		rccsc.logger.Error("unable-to-clean-up-resource-config-check-sessions-for-paused-and-inactive-resources", err)
		return err
	}

	return nil
}
