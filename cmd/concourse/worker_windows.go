package main

import (
	"github.com/concourse/atc"
	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

type GardenBackend struct{}

func (cmd WorkerCommand) lessenRequirements(command *flags.Command) {}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, args []string) (atc.Worker, ifrit.Runner, error) {
	return cmd.houdiniRunner(logger, "windows")
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	return cmd.naiveBaggageclaimRunner(logger)
}
