package main

import (
	"errors"
	"fmt"
	"io/ioutil"
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

	// must be readable by other users so unprivileged containers can run their
	// own `initc' process
	err = os.MkdirAll(depotDir, 0755)
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

	var resourceTypes []atc.WorkerResourceType

	resourcesDir := filepath.Join(linux, "resources")
	resourceImagesDir := filepath.Join(linux, "resource-images")

	tarBin := filepath.Join(binDir, "tar")

	infos, err := ioutil.ReadDir(resourcesDir)
	if err == nil {
		for _, info := range infos {
			archive := filepath.Join(resourcesDir, info.Name())
			resourceType := info.Name()

			imageDir := filepath.Join(resourceImagesDir, resourceType)

			err := os.RemoveAll(imageDir)
			if err != nil {
				return err
			}

			err = os.MkdirAll(imageDir, 0755)
			if err != nil {
				return err
			}

			tar := exec.Command(tarBin, "-zxf", archive, "-C", imageDir)
			tar.Stdout = os.Stdout
			tar.Stderr = os.Stderr

			err = tar.Run()
			if err != nil {
				return err
			}

			resourceTypes = append(resourceTypes, atc.WorkerResourceType{
				Type:  resourceType,
				Image: imageDir,
			})
		}
	}

	worker := atc.Worker{
		Name:     "foo",
		Platform: "linux",
		Tags:     []string{},

		ResourceTypes: resourceTypes,
	}

	beacon := Beacon{
		Config: cmd.TSA,
	}

	var beaconRunner ifrit.RunFunc
	if cmd.PeerIP != "" {
		worker.GardenAddr = fmt.Sprintf("%s:%d", cmd.PeerIP, cmd.BindPort)
		beaconRunner = beacon.Register
	} else {
		worker.GardenAddr = fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
		beaconRunner = beacon.Forward
	}

	beacon.Worker = worker

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, grouper.Members{
		{"garden", cmdRunner{gardenCmd}},
		{"beacon", beaconRunner},
	}))

	return <-ifrit.Invoke(runner).Wait()
}
