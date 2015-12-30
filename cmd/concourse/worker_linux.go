package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/vito/concourse-bin/bindata"
)

var ErrNotRoot = errors.New("worker must be run as root")

func (cmd *WorkerCommand) Execute(args []string) error {
	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	if currentUser.Uid != "0" {
		return ErrNotRoot
	}

	err = bindata.RestoreAssets(cmd.WorkDir, "linux")
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

	gardenAddr := fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)

	gardenArgs := []string{
		"-listenNetwork", "tcp",
		"-listenAddr", gardenAddr,
		"-bin", binDir,
		"-depot", depotDir,
		"-graph", graphDir,
		"-snapshots", snapshotsDir,
		"-stateDir", stateDir,
		"-allowHostAccess",
	}

	gardenArgs = append(gardenArgs, args...)

	gardenCmd := exec.Command(gardenBin, gardenArgs...)
	gardenCmd.Stdout = os.Stdout
	gardenCmd.Stderr = os.Stderr

	var gardenPeerAddr string
	if cmd.PeerIP != "" {
		gardenPeerAddr = fmt.Sprintf("%s:%d", cmd.PeerIP, cmd.BindPort)
	}

	beacon := Beacon{
		Worker: atc.Worker{
			Name:     "foo",
			Platform: "linux",
			Tags:     []string{},

			GardenAddr: gardenPeerAddr,

			BaggageclaimURL: "",

			ResourceTypes: []atc.WorkerResourceType{},
		},
		Config: cmd.TSA,
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, grouper.Members{
		{"garden", cmdRunner{gardenCmd}},
		{"beacon", ifrit.RunFunc(beacon.Register)},
	}))

	return <-ifrit.Invoke(runner).Wait()
}
