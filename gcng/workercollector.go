package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type workerCollector struct {
	logger          lager.Logger
	workerLifecycle dbng.WorkerLifecycle
}

func NewWorkerCollector(
	logger lager.Logger,
	workerLifecycle dbng.WorkerLifecycle,
) Collector {
	return &workerCollector{
		logger:          logger,
		workerLifecycle: workerLifecycle,
	}
}

func (wc *workerCollector) Run() error {
	logger := wc.logger.Session("collect")

	affected, err := wc.workerLifecycle.StallUnresponsiveWorkers()
	if err != nil {
		logger.Error("failed-to-mark-workers-as-stalled", err)
		return err
	}

	if len(affected) > 0 {
		logger.Debug("stalled", lager.Data{"count": len(affected), "workers": affected})
	}

	affected, err = wc.workerLifecycle.DeleteFinishedRetiringWorkers()
	if err != nil {
		logger.Error("failed-to-delete-finished-retiring-workers", err)
		return err
	}

	if len(affected) > 0 {
		logger.Debug("retired", lager.Data{"count": len(affected), "workers": affected})
	}

	affected, err = wc.workerLifecycle.LandFinishedLandingWorkers()
	if err != nil {
		logger.Error("failed-to-land-finished-landing-workers", err)
		return err
	}

	if len(affected) > 0 {
		logger.Debug("landed", lager.Data{"count": len(affected), "workers": affected})
	}

	return nil
}
