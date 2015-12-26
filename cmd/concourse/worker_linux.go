package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/vito/concourse-bin/bindata"
)

func (cmd *WorkerCommand) Execute(args []string) error {
	err := bindata.RestoreAssets(cmd.WorkDir, "linux")
	if err != nil {
		return err
	}

	linux := filepath.Join(cmd.WorkDir, "linux")

	gardenBin := filepath.Join(linux, "garden-linux")
	binDir := filepath.Join(linux, "bin")
	depotDir := filepath.Join(linux, "depot")
	graphDir := filepath.Join(linux, "graph")
	snapshotsDir := filepath.Join(linux, "snapshots")
	stateDir := filepath.Join(linux, "state")

	err = os.MkdirAll(depotDir, 0700)
	if err != nil {
		return err
	}

	err = os.MkdirAll(graphDir, 0700)
	if err != nil {
		return err
	}

	return run(exec.Command(
		gardenBin,
		"-listenNetwork", "tcp",
		"-listenAddr", "0.0.0.0:7777",
		"-allowHostAccess",
		"-bin", binDir,
		"-depot", depotDir,
		"-graph", graphDir,
		"-snapshots", snapshotsDir,
		"-stateDir", stateDir,
	))
}
