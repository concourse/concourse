// +build !linux

package main

import (
	"runtime"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

type GardenBackend struct{}

type Certs struct{}

func (cmd WorkerCommand) lessenRequirements(prefix string, command *flags.Command) {
	// created in the work-dir
	command.FindOptionByLongName(prefix + "baggageclaim-volumes").Required = false
	command.FindOptionByLongName(prefix + "tsa-worker-private-key").Required = false
}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger) (atc.Worker, ifrit.Runner, error) {
	worker := cmd.Worker.Worker()
	worker.Platform = runtime.GOOS
	var err error
	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	runner, err := cmd.houdiniRunner(logger)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	return worker, runner, nil
}
