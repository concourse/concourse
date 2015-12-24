package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/vito/concourse-bin/bindata"
)

func (cmd *ConcourseCommand) garden() (*exec.Cmd, error) {
	err := bindata.RestoreAssets(cmd.WorkDir, "linux")
	if err != nil {
		return nil, err
	}

	gardenBin := filepath.Join(cmd.WorkDir, "linux", "garden-linux")
	binDir := filepath.Join(cmd.WorkDir, "linux", "bin")
	depotDir := filepath.Join(cmd.WorkDir, "linux", "depot")
	graphDir := filepath.Join(cmd.WorkDir, "linux", "graph")
	snapshotsDir := filepath.Join(cmd.WorkDir, "linux", "snapshots")
	stateDir := filepath.Join(cmd.WorkDir, "linux", "state")

	err = os.MkdirAll(depotDir, 0700)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(graphDir, 0700)
	if err != nil {
		return nil, err
	}

	return exec.Command(
		gardenBin,
		"-listenNetwork", "tcp",
		"-listenAddr", "0.0.0.0:7777",
		"-allowHostAccess",
		"-bin", binDir,
		"-depot", depotDir,
		"-graph", graphDir,
		"-snapshots", snapshotsDir,
		"-stateDir", stateDir,
	), nil
}
