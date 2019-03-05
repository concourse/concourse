package worker

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
)

// ContainerSweeper is an ifrit.Runner that periodically reports and
// garbage-collects a worker's containers
type ContainerSweeper struct {
	Logger       lager.Logger
	Interval     time.Duration
	TSAClient    TSAClient
	GardenClient garden.Client
}

func (cmd *ContainerSweeper) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
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

func (cmd *ContainerSweeper) sweep(logger lager.Logger) {
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
}

