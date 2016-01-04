package main

import (
	"os"

	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/pivotal-golang/lager"
)

type gardenServerRunner struct {
	logger       lager.Logger
	gardenServer *server.GardenServer
}

func (runner gardenServerRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := runner.gardenServer.Start()
	if err != nil {
		return err
	}

	close(ready)

	runner.logger.Info("started")

	select {
	case <-signals:
		runner.gardenServer.Stop()
		return nil
	}
}
