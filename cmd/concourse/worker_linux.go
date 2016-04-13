package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/baggageclaim/fs"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/concourse/bin/bindata"
)

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, args []string) (atc.Worker, ifrit.Runner, error) {
	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	err = bindata.RestoreAssets(cmd.WorkDir, "linux")
	if err != nil {
		return atc.Worker{}, nil, err
	}

	linux := filepath.Join(cmd.WorkDir, "linux")

	btrfsToolsDir := filepath.Join(linux, "btrfs")
	err = os.Setenv("PATH", btrfsToolsDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err != nil {
		return atc.Worker{}, nil, err
	}

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

	busyboxDir, err := cmd.extractBusybox(linux)

	gardenArgs := []string{
		"-listenNetwork", "tcp",
		"-listenAddr", cmd.bindAddr(),
		"-bin", binDir,
		"-depot", depotDir,
		"-graph", graphDir,
		"-snapshots", snapshotsDir,
		"-stateDir", stateDir,
		"-rootfs", busyboxDir,
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

	return worker, &cmd.Guardian, nil
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	if output, err := exec.Command("modprobe", "btrfs").CombinedOutput(); err != nil {
		logger.Error("btrfs-unavailable-falling-back-to-naive", err, lager.Data{
			"modprobe-log": string(output),
		})
		return cmd.naiveBaggageclaimRunner(logger)
	}

	volumesImage := filepath.Join(cmd.WorkDir, "volumes.img")
	volumesDir := filepath.Join(cmd.WorkDir, "volumes")

	err := os.MkdirAll(volumesDir, 0755)
	if err != nil {
		return nil, err
	}

	var fsStat syscall.Statfs_t
	err = syscall.Statfs(volumesDir, &fsStat)
	if err != nil {
		return nil, fmt.Errorf("failed to stat volumes filesystem: %s", err)
	}

	filesystem := fs.New(logger.Session("fs"), volumesImage, volumesDir)

	err = filesystem.Create(fsStat.Blocks * uint64(fsStat.Bsize))
	if err != nil {
		return nil, fmt.Errorf("failed to set up volumes filesystem: %s", err)
	}

	bc := &baggageclaimcmd.BaggageclaimCommand{
		BindIP:   baggageclaimcmd.IPFlag(cmd.Baggageclaim.BindIP),
		BindPort: cmd.Baggageclaim.BindPort,

		VolumesDir: baggageclaimcmd.DirFlag(volumesDir),

		Driver: "btrfs",

		ReapInterval: cmd.Baggageclaim.ReapInterval,

		Metrics: cmd.Metrics,
	}

	return bc.Runner(nil)
}

func (cmd *WorkerCommand) extractBusybox(linux string) (string, error) {
	archive := filepath.Join(linux, "busybox.tar.gz")

	busyboxDir := filepath.Join(linux, "busybox")
	err := os.MkdirAll(busyboxDir, 0755)
	if err != nil {
		return "", err
	}

	tarBin := filepath.Join(linux, "bin", "tar")
	tar := exec.Command(tarBin, "-zxf", archive, "-C", busyboxDir)
	tar.Stdout = os.Stdout
	tar.Stderr = os.Stderr

	err = tar.Run()
	if err != nil {
		return "", err
	}

	return busyboxDir, nil
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
