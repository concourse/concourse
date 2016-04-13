// +build windows darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/vito/houdini"
)

func (cmd *WorkerCommand) houdiniRunner(logger lager.Logger, platform string) (atc.Worker, ifrit.Runner, error) {
	depotDir := filepath.Join(cmd.WorkDir, "containers")

	err := os.MkdirAll(depotDir, 0755)
	if err != nil {
		return atc.Worker{}, nil, fmt.Errorf("failed to create depot dir: %s", err)
	}

	backend := houdini.NewBackend(depotDir)

	server := server.New(
		"tcp",
		cmd.bindAddr(),
		0,
		backend,
		logger,
	)

	worker := atc.Worker{
		Platform: platform,
		Tags:     cmd.Tags,
	}

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	return worker, gardenServerRunner{logger, server}, nil
}

func (cmd *WorkerCommand) bindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP.IP(), cmd.BindPort)
}
