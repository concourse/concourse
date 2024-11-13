package workercmd

import (
	"net"
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager/v3"
)

type gardenServerRunner struct {
	logger        lager.Logger
	backend       garden.Backend
	gardenServer  *server.GardenServer
	listenNetwork string
	listenAddr    string
}

func newGardenServerRunner(
	listenNetwork, listenAddr string,
	containerGraceTime time.Duration,
	backend garden.Backend,
	logger lager.Logger,
) gardenServerRunner {
	return gardenServerRunner{
		gardenServer:  server.New(listenNetwork, listenAddr, containerGraceTime, 0, backend, logger),
		listenNetwork: listenNetwork,
		listenAddr:    listenAddr,
		backend:       backend,
		logger:        logger,
	}
}

func (runner gardenServerRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	if err := runner.backend.Start(); err != nil {
		return err
	}
	if err := runner.gardenServer.SetupBomberman(); err != nil {
		return err
	}
	listener, err := net.Listen(runner.listenNetwork, runner.listenAddr)
	if err != nil {
		return err
	}
	errs := make(chan error, 1)
	go func() {
		errs <- runner.gardenServer.Serve(listener)
	}()

	close(ready)

	runner.logger.Info("started")
	defer runner.logger.Info("stopped")

	select {
	case <-signals:
		runner.logger.Info("signaled")
		runner.gardenServer.Stop()
		return nil
	case err := <-errs:
		return err
	}
}
