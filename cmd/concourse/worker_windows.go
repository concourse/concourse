package main

import (
	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, args []string) (atc.Worker, ifrit.Runner, error) {
	return cmd.houdiniRunner(logger, "windows")
}
