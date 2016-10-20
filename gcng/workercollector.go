package gcng

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type WorkerCollector interface {
	Run() error
}

type workerCollector struct {
	logger        lager.Logger
	workerFactory dbng.WorkerFactory
}

func NewWorkerCollector(
	logger lager.Logger,
	workerFactory dbng.WorkerFactory,
) WorkerCollector {
	return &workerCollector{
		logger:        logger,
		workerFactory: workerFactory,
	}
}

func (wc *workerCollector) Run() error {
	affected, err := wc.workerFactory.StallUnresponsiveWorkers()
	if err != nil {
		wc.logger.Error("failed-to-mark-workers-as-stalled", err)
		return err
	}

	wc.logger.Debug(fmt.Sprintf("stalled-%d-workers", len(affected)), lager.Data{"stalled-workers": affected})

	return nil
}
