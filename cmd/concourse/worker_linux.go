package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/guardian/guardiancmd"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/baggageclaim/fs"
	"github.com/concourse/bin/bindata"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

const btrfsFSType = 0x9123683e

type GardenBackend guardiancmd.ServerCommand

func (cmd WorkerCommand) lessenRequirements(command *flags.Command) {
	command.FindOptionByLongName("garden-bind-ip").Default = []string{"7777"}

	command.FindOptionByLongName("garden-depot").Required = false
	command.FindOptionByLongName("garden-graph").Required = false
	command.FindOptionByLongName("garden-runc-bin").Required = false
	command.FindOptionByLongName("garden-dadoo-bin").Required = false
	command.FindOptionByLongName("garden-init-bin").Required = false
	command.FindOptionByLongName("garden-nstar-bin").Required = false
	command.FindOptionByLongName("garden-tar-bin").Required = false

	command.FindOptionByLongName("baggageclaim-volumes").Required = false
	command.FindOptionByLongName("baggageclaim-driver").Default = []string{"btrfs"}
}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, args []string) (atc.Worker, ifrit.Runner, error) {
	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	assetsDir, err := cmd.restoreVersionedAssets(logger.Session("unpack-assets"))
	if err != nil {
		return atc.Worker{}, nil, err
	}

	depotDir := filepath.Join(cmd.WorkDir, "depot")

	// must be readable by other users so unprivileged containers can run their
	// own `initc' process
	err = os.MkdirAll(depotDir, 0755)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	cmd.Garden.Server.BindIP = guardiancmd.IPFlag(cmd.BindIP)

	cmd.Garden.Containers.Dir = depotDir

	cmd.Garden.Bin.Runc = filepath.Join(assetsDir, "bin", "runc")
	cmd.Garden.Bin.Dadoo = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "dadoo"))
	cmd.Garden.Bin.Init = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "init"))
	cmd.Garden.Bin.NSTar = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "nstar"))
	cmd.Garden.Bin.Tar = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "tar"))

	iptablesDir := filepath.Join(assetsDir, "iptables")
	cmd.Garden.Bin.IPTables = guardiancmd.FileFlag(filepath.Join(iptablesDir, "sbin", "iptables"))

	cmd.Garden.Network.AllowHostAccess = true

	worker := atc.Worker{
		Platform: "linux",
		Tags:     cmd.Tags,
		Team:     cmd.TeamName,

		HTTPProxyURL:  cmd.HTTPProxy.String(),
		HTTPSProxyURL: cmd.HTTPSProxy.String(),
		NoProxy:       strings.Join(cmd.NoProxy, ","),
		StartTime:     time.Now().Unix(),
	}

	worker.ResourceTypes, err = cmd.extractResources(
		logger.Session("extract-resources"),
		assetsDir,
	)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	runner := guardiancmd.ServerCommand(cmd.Garden)
	return worker, &runner, nil
}

func (cmd *WorkerCommand) restoreVersionedAssets(logger lager.Logger) (string, error) {
	assetsDir := filepath.Join(cmd.WorkDir, Version)

	restoredDir := filepath.Join(assetsDir, "linux")

	okMarker := filepath.Join(assetsDir, "ok")

	_, err := os.Stat(okMarker)
	if err == nil {
		logger.Info("already-done")
		return restoredDir, nil
	}

	logger.Info("unpacking")

	err = bindata.RestoreAssets(assetsDir, "linux")
	if err != nil {
		logger.Error("failed-to-unpack", err)
		return "", err
	}

	ok, err := os.Create(okMarker)
	if err != nil {
		logger.Error("failed-to-create-ok-marker", err)
		return "", err
	}

	err = ok.Close()
	if err != nil {
		logger.Error("failed-to-close-ok-marker", err)
		return "", err
	}

	logger.Info("done")

	return restoredDir, nil
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	volumesImage := filepath.Join(cmd.WorkDir, "volumes.img")
	volumesDir := filepath.Join(cmd.WorkDir, "volumes")

	assetsDir, err := cmd.restoreVersionedAssets(logger.Session("unpack-assets"))
	if err != nil {
		return nil, err
	}
	btrfsToolsDir := filepath.Join(assetsDir, "btrfs")

	err = os.MkdirAll(volumesDir, 0755)
	if err != nil {
		return nil, err
	}

	var fsStat syscall.Statfs_t
	err = syscall.Statfs(volumesDir, &fsStat)
	if err != nil {
		return nil, fmt.Errorf("failed to stat volumes filesystem: %s", err)
	}

	if fsStat.Type != btrfsFSType {
		filesystem := fs.New(logger.Session("fs"), volumesImage, volumesDir, filepath.Join(btrfsToolsDir, "mkfs.btrfs"))

		err = filesystem.Create(fsStat.Blocks * uint64(fsStat.Bsize))
		if err != nil {
			logger.Error("falling-back-on-naive-driver", err)
			return cmd.naiveBaggageclaimRunner(logger)
		}
	}

	cmd.Baggageclaim.Metrics = cmd.Metrics
	cmd.Baggageclaim.VolumesDir = baggageclaimcmd.DirFlag(volumesDir)

	cmd.Baggageclaim.MkfsBin = filepath.Join(btrfsToolsDir, "mkfs.btrfs")
	cmd.Baggageclaim.BtrfsBin = filepath.Join(btrfsToolsDir, "btrfs")

	return cmd.Baggageclaim.Runner(nil)
}

func (cmd *WorkerCommand) extractResources(logger lager.Logger, assetsDir string) ([]atc.WorkerResourceType, error) {
	var resourceTypes []atc.WorkerResourceType

	resourcesDir := filepath.Join(assetsDir, "resources")

	infos, err := ioutil.ReadDir(resourcesDir)
	if err != nil {
		logger.Error("failed-to-list-resource-assets", err)
		return nil, err
	}

	for _, info := range infos {
		resourceType := info.Name()

		workerResourceType, err := cmd.extractResource(
			logger.Session("extract", lager.Data{"resource-type": resourceType}),
			assetsDir,
			resourcesDir,
			resourceType,
		)
		if err != nil {
			logger.Error("failed-to-extract-resource", err)
			return nil, err
		}

		resourceTypes = append(resourceTypes, workerResourceType)
	}

	return resourceTypes, nil
}

func (cmd *WorkerCommand) extractResource(
	logger lager.Logger,
	assetsDir string,
	resourcesDir string,
	resourceType string,
) (atc.WorkerResourceType, error) {
	resourceImagesDir := filepath.Join(assetsDir, "resource-images")
	tarBin := filepath.Join(assetsDir, "bin", "tar")

	archive := filepath.Join(resourcesDir, resourceType, "rootfs.tar.gz")

	extractedDir := filepath.Join(resourceImagesDir, resourceType)

	rootfsDir := filepath.Join(extractedDir, "rootfs")
	okMarker := filepath.Join(extractedDir, "ok")

	var version string
	versionFile, err := os.Open(filepath.Join(resourcesDir, resourceType, "version"))
	if err != nil {
		logger.Error("failed-to-read-version", err)
		return atc.WorkerResourceType{}, err
	}

	_, err = fmt.Fscanf(versionFile, "%s", &version)
	if err != nil {
		logger.Error("failed-to-parse-version", err)
		return atc.WorkerResourceType{}, err
	}

	defer versionFile.Close()

	_, err = os.Stat(okMarker)
	if err == nil {
		logger.Info("already-extracted")
	} else {
		logger.Info("extracting")

		err := os.RemoveAll(rootfsDir)
		if err != nil {
			logger.Error("failed-to-clear-out-existing-rootfs", err)
			return atc.WorkerResourceType{}, err
		}

		err = os.MkdirAll(rootfsDir, 0755)
		if err != nil {
			logger.Error("failed-to-create-rootfs-dir", err)
			return atc.WorkerResourceType{}, err
		}

		tar := exec.Command(tarBin, "-zxf", archive, "-C", rootfsDir)

		output, err := tar.CombinedOutput()
		if err != nil {
			logger.Error("failed-to-extract-resource", err, lager.Data{
				"output": string(output),
			})
			return atc.WorkerResourceType{}, err
		}

		ok, err := os.Create(okMarker)
		if err != nil {
			logger.Error("failed-to-create-ok-marker", err)
			return atc.WorkerResourceType{}, err
		}

		err = ok.Close()
		if err != nil {
			logger.Error("failed-to-close-ok-marker", err)
			return atc.WorkerResourceType{}, err
		}
	}

	return atc.WorkerResourceType{
		Type:    resourceType,
		Image:   rootfsDir,
		Version: version,
	}, nil
}
