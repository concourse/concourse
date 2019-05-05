package gc

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

type workerCollector struct {
	workerLifecycle db.WorkerLifecycle
}

func NewWorkerCollector(workerLifecycle db.WorkerLifecycle) Collector {
	return &workerCollector{
		workerLifecycle: workerLifecycle,
	}
}

// TODO - gc duration?
func (wc *workerCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("worker-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	affected, err := wc.workerLifecycle.DeleteUnresponsiveEphemeralWorkers()
	if err != nil {
		logger.Error("failed-to-remove-dead-ephemeral-workers", err)
		return err
	}

	if len(affected) > 0 {
		logger.Info("ephemeral-workers-removed", lager.Data{"count": len(affected), "workers": affected})

		// workers_ephemeral_removed

	}

	affected, err = wc.workerLifecycle.StallUnresponsiveWorkers()
	if err != nil {
		logger.Error("failed-to-mark-workers-as-stalled", err)
		return err
	}

	if len(affected) > 0 {
		logger.Info("marked-workers-as-stalled", lager.Data{"count": len(affected), "workers": affected})

		// workers_stalled

	}

	affected, err = wc.workerLifecycle.DeleteFinishedRetiringWorkers()
	if err != nil {
		logger.Error("failed-to-delete-finished-retiring-workers", err)
		return err
	}

	if len(affected) > 0 {

		// workers_retired

		logger.Info("marked-workers-as-retired", lager.Data{"count": len(affected), "workers": affected})
	}

	affected, err = wc.workerLifecycle.LandFinishedLandingWorkers()
	if err != nil {
		logger.Error("failed-to-land-finished-landing-workers", err)
		return err
	}

	if len(affected) > 0 {

		// workers_landed

		logger.Info("marked-workers-as-landed", lager.Data{"count": len(affected), "workers": affected})
	}

	return nil
}
