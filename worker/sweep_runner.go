package worker

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
)

// SweepRunner is an ifrit.Runner that periodically reports and
// garbage-collects a worker's containers and volumes.
type SweepRunner struct {
	Logger lager.Logger

	Interval time.Duration

	TSAClient TSAClient

	GardenClient       garden.Client
	BaggageclaimClient baggageclaim.Client
}

func (cmd *SweepRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
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

func (cmd *SweepRunner) sweep(logger lager.Logger) {
	ctx := lagerctx.NewContext(context.Background(), logger)

	containers, err := cmd.GardenClient.Containers(garden.Properties{})
	if err != nil {
		logger.Error("failed-to-list-containers", err)
	} else {
		handles := []string{}
		for _, container := range containers {
			handles = append(handles, container.Handle())
		}

		err := cmd.TSAClient.ReportContainers(ctx, handles)
		if err != nil {
			logger.Error("failed-to-report-containers", err)
		}
	}

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

	containerHandles, err := cmd.TSAClient.ContainersToDestroy(ctx)
	if err != nil {
		logger.Error("failed-to-sweep-containers", err)
	} else {
		for _, handle := range containerHandles {
			err := cmd.GardenClient.Destroy(handle)
			if err != nil {
				logger.Error("failed-to-destroy-container", err)
			}
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
