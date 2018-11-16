package main

import (
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"os"
)

func NewLoggingRunner(logger lager.Logger, runner ifrit.Runner) ifrit.Runner {
	return &loggingRunner{
		logger: logger,
		runner: runner,
	}
}

type loggingRunner struct {
	logger lager.Logger
	runner ifrit.Runner
}

func (lr *loggingRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := lr.runner.Run(signals, ready)
	lr.logger.Info("logging-runner-exited", lager.Data{"err": err})
	return err
}
