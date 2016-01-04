package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/vito/houdini"
)

var ErrNotRoot = errors.New("worker must be run as root")

func (cmd *WorkerCommand) gardenRunner(args []string) (atc.Worker, ifrit.Runner, error) {
	logger := lager.NewLogger("garden")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	depotDir := filepath.Join(cmd.WorkDir, "containers")

	err = os.MkdirAll(depotDir, 0755)
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
		Platform: "darwin",
		Tags:     cmd.Tags,
	}

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	return worker, gardenServerRunner{logger, server}, nil
}
