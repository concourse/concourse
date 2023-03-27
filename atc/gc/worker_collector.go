package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

type workerCollector struct {
	workerLifecycle db.WorkerLifecycle
}

func NewWorkerCollector(workerLifecycle db.WorkerLifecycle) *workerCollector {
	return &workerCollector{
		workerLifecycle: workerLifecycle,
	}
}

func (wc *workerCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("worker-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.WorkerCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	affected, err := wc.workerLifecycle.DeleteUnresponsiveEphemeralWorkers()
	if err != nil {
		logger.Error("failed-to-remove-dead-ephemeral-workers", err)
		return err
	}

	if len(affected) > 0 {
		logger.Info("ephemeral-workers-removed", lager.Data{"count": len(affected), "workers": affected})
	}

	affected, err = wc.workerLifecycle.StallUnresponsiveWorkers()
	if err != nil {
		logger.Error("failed-to-mark-workers-as-stalled", err)
		return err
	}

	if len(affected) > 0 {
		logger.Info("marked-workers-as-stalled", lager.Data{"count": len(affected), "workers": affected})
	}

	affected, err = wc.workerLifecycle.DeleteFinishedRetiringWorkers()
	if err != nil {
		logger.Error("failed-to-delete-finished-retiring-workers", err)
		return err
	}

	if len(affected) > 0 {
		logger.Info("marked-workers-as-retired", lager.Data{"count": len(affected), "workers": affected})
	}

	affected, err = wc.workerLifecycle.LandFinishedLandingWorkers()
	if err != nil {
		logger.Error("failed-to-land-finished-landing-workers", err)
		return err
	}

	if len(affected) > 0 {
		logger.Info("marked-workers-as-landed", lager.Data{"count": len(affected), "workers": affected})
	}

	workerStateByName, err := wc.workerLifecycle.GetWorkerStateByName()

	if err != nil {
		logger.Error("failed-to-get-workers-states-for-metrics", err)
	} else {
		metric.WorkersState{
			WorkerStateByName: workerStateByName,
		}.Emit(logger, metric.Metrics)
	}

	return nil
}
