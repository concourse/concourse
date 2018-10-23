package gc

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

type workerCollector struct {
	workerLifecycle db.WorkerLifecycle
}

func NewWorkerCollector(workerLifecycle db.WorkerLifecycle) Collector {
	return &workerCollector{
		workerLifecycle: workerLifecycle,
	}
}

func (wc *workerCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("worker-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	affected, err := wc.workerLifecycle.DeleteUnresponsiveEphemeralWorkers()
	if err != nil {
		logger.Error("failed-to-remove-dead-ephemeral-workers", err)
		return err
	}

	affected, err = wc.workerLifecycle.StallUnresponsiveWorkers()
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

	workerStateByName, err := wc.workerLifecycle.GetWorkerStateByName()

	if err != nil {
		logger.Error("failed-to-get-workers-states-for-metrics", err)
	} else {
		metric.WorkersState{
			WorkerStateByName: workerStateByName,
		}.Emit(logger)
	}

	return nil
}
