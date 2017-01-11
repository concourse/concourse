package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type workerCollector struct {
	logger        lager.Logger
	workerFactory dbng.WorkerFactory
}

func NewWorkerCollector(
	logger lager.Logger,
	workerFactory dbng.WorkerFactory,
) Collector {
	return &workerCollector{
		logger:        logger,
		workerFactory: workerFactory,
	}
}

func (wc *workerCollector) Run() error {
	logger := wc.logger.Session("collect")

	affected, err := wc.workerFactory.StallUnresponsiveWorkers()
	if err != nil {
		logger.Error("failed-to-mark-workers-as-stalled", err)
		return err
	}

	if len(affected) > 0 {
		workerNames := make([]string, len(affected))
		for i, w := range affected {
			workerNames[i] = w.Name
		}

		logger.Debug("stalled", lager.Data{"count": len(affected), "workers": workerNames})
	}

	err = wc.workerFactory.DeleteFinishedRetiringWorkers()
	if err != nil {
		logger.Error("failed-to-delete-finished-retiring-workers", err)
		return err
	}

	logger.Debug("deleted-finished-retiring-workers")

	err = wc.workerFactory.LandFinishedLandingWorkers()
	if err != nil {
		logger.Error("failed-to-land-finished-landing-workers", err)
		return err
	}

	logger.Debug("landed-finished-landing-workers")

	return nil
}
