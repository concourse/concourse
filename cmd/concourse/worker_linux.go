package main

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
	"github.com/vito/concourse-bin/bindata"
)

var ErrNotRoot = errors.New("worker must be run as root")

func (cmd *WorkerCommand) gardenRunner(args []string) (atc.Worker, ifrit.Runner, error) {
	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	err = bindata.RestoreAssets(cmd.WorkDir, "linux")
	if err != nil {
		return atc.Worker{}, nil, err
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
		return atc.Worker{}, nil, err
	}

	err = os.MkdirAll(graphDir, 0700)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	gardenArgs := []string{
		"-listenNetwork", "tcp",
		"-listenAddr", cmd.bindAddr(),
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

	worker := atc.Worker{
		Platform: "linux",
		Tags:     cmd.Tags,
	}

	worker.ResourceTypes, err = cmd.extractResources(linux)

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	return worker, cmdRunner{gardenCmd}, nil
}

func (cmd *WorkerCommand) extractResources(linux string) ([]atc.WorkerResourceType, error) {
	var resourceTypes []atc.WorkerResourceType

	binDir := filepath.Join(linux, "bin")
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
				return nil, err
			}

			err = os.MkdirAll(imageDir, 0755)
			if err != nil {
				return nil, err
			}

			tar := exec.Command(tarBin, "-zxf", archive, "-C", imageDir)
			tar.Stdout = os.Stdout
			tar.Stderr = os.Stderr

			err = tar.Run()
			if err != nil {
				return nil, err
			}

			resourceTypes = append(resourceTypes, atc.WorkerResourceType{
				Type:  resourceType,
				Image: imageDir,
			})
		}
	}

	return resourceTypes, nil
}
