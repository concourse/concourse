package cmd

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
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
	if err != nil {
		lr.logger.Error("logging-runner-exited", err)
	} else {
		lr.logger.Info("logging-runner-exited")
	}

	return err
}
