package main

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

type GardenBackend struct{}

func (cmd WorkerCommand) lessenRequirements(command *flags.Command) {
	command.FindOptionByLongName("baggageclaim-volumes").Required = false
}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, args []string) (atc.Worker, ifrit.Runner, error) {
	return cmd.houdiniRunner(logger, "darwin")
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	return cmd.naiveBaggageclaimRunner(logger)
}
