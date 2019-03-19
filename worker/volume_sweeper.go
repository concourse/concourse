package worker

import (
	"context"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
)

// volumeSweeper is an ifrit.Runner that periodically reports and
// garbage-collects a worker's volumes
type volumeSweeper struct {
	logger             lager.Logger
	interval           time.Duration
	tsaClient          TSAClient
	baggageclaimClient baggageclaim.Client
	maxInFlight        uint16
}

func NewVolumeSweeper(
	logger lager.Logger,
	sweepInterval time.Duration,
	tsaClient TSAClient,
	bcClient baggageclaim.Client,
	maxInFlight uint16,
) *volumeSweeper {
	return &volumeSweeper{
		logger:             logger,
		interval:           sweepInterval,
		tsaClient:          tsaClient,
		baggageclaimClient: bcClient,
		maxInFlight:        maxInFlight,
	}
}

func (sweeper *volumeSweeper) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	timer := time.NewTicker(sweeper.interval)

	close(ready)

	for {
		select {
		case <-timer.C:
			sweeper.sweep(sweeper.logger.Session("tick"))

		case sig := <-signals:
			sweeper.logger.Info("sweep-cancelled-by-signal", lager.Data{"signal": sig})
			return nil
		}
	}
}

func (sweeper *volumeSweeper) sweep(logger lager.Logger) {
	ctx := lagerctx.NewContext(context.Background(), logger)

	volumes, err := sweeper.baggageclaimClient.ListVolumes(logger.Session("list-volumes"), baggageclaim.VolumeProperties{})
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
	} else {
		handles := []string{}
		for _, volume := range volumes {
			handles = append(handles, volume.Handle())
		}

		err := sweeper.tsaClient.ReportVolumes(ctx, handles)
		if err != nil {
			logger.Error("failed-to-report-volumes", err)
		}
	}

	volumeHandles, err := sweeper.tsaClient.VolumesToDestroy(ctx)
	if err != nil {
		logger.Error("failed-to-get-volumes-to-destroy", err)
	} else {
		var wg sync.WaitGroup
		maxInFlight := make(chan int, sweeper.maxInFlight)

		for _, handle := range volumeHandles {
			maxInFlight <- 1
			wg.Add(1)

			go func(handle string) {
				err := sweeper.baggageclaimClient.DestroyVolume(logger.Session("destroy-volumes"), handle)
				if err != nil {
					logger.WithData(lager.Data{"handle": handle}).Error("failed-to-destroy-volume", err)
				}

				<-maxInFlight
				wg.Done()
			}(handle)
		}
		wg.Wait()
	}
}
