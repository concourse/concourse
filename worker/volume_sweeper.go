package worker

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
)

// VolumeSweeper is an ifrit.Runner that periodically reports and
// garbage-collects a worker's volumes
type VolumeSweeper struct {
	Logger             lager.Logger
	Interval           time.Duration
	TSAClient          TSAClient
	BaggageclaimClient baggageclaim.Client
}

func (cmd *VolumeSweeper) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	timer := time.NewTicker(cmd.Interval)

	close(ready)

	for {
		select {
		case <-timer.C:
			cmd.sweep(cmd.Logger.Session("tick"))

		case sig := <-signals:
			cmd.Logger.Info("sweep-cancelled-by-signal", lager.Data{"signal": sig})
			return nil
		}
	}
}

func (cmd *VolumeSweeper) sweep(logger lager.Logger) {
	ctx := lagerctx.NewContext(context.Background(), logger)

	volumes, err := cmd.BaggageclaimClient.ListVolumes(logger.Session("list-volumes"), baggageclaim.VolumeProperties{})
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
	} else {
		handles := []string{}
		for _, volume := range volumes {
			handles = append(handles, volume.Handle())
		}

		err := cmd.TSAClient.ReportVolumes(ctx, handles)
		if err != nil {
			logger.Error("failed-to-report-volumes", err)
		}
	}

	volumeHandles, err := cmd.TSAClient.VolumesToDestroy(ctx)
	if err != nil {
		logger.Error("failed-to-sweep-volumes", err)
	} else if len(volumeHandles) > 0 {
		err := cmd.BaggageclaimClient.DestroyVolumes(logger.Session("destroy-volumes"), volumeHandles)
		if err != nil {
			logger.Error("failed-to-destroy-volumes", err)
		}
	}
}
