// +build !linux

package main

import (
	"runtime"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

type GardenBackend struct{}

type Certs struct{}

func (cmd WorkerCommand) lessenRequirements(prefix string, command *flags.Command) {
	// created in the work-dir
	command.FindOptionByLongName(prefix + "baggageclaim-volumes").Required = false
}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, hasAssets bool) (atc.Worker, ifrit.Runner, error) {
	return cmd.houdiniRunner(logger, runtime.GOOS)
}
