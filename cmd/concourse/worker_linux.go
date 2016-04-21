package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cloudfoundry-incubator/guardian/guardiancmd"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/baggageclaim/fs"
	"github.com/concourse/bin/bindata"
	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

const btrfsFSType = 0x9123683e

type GardenBackend guardiancmd.GuardianCommand

func (cmd WorkerCommand) lessenRequirements(command *flags.Command) {
	command.FindOptionByLongName("garden-depot").Required = false
	command.FindOptionByLongName("garden-graph").Required = false
	command.FindOptionByLongName("garden-runc-bin").Required = false
	command.FindOptionByLongName("garden-dadoo-bin").Required = false
	command.FindOptionByLongName("garden-init-bin").Required = false
	command.FindOptionByLongName("garden-iodaemon-bin").Required = false
	command.FindOptionByLongName("garden-kawasaki-bin").Required = false
	command.FindOptionByLongName("garden-nstar-bin").Required = false
	command.FindOptionByLongName("garden-tar-bin").Required = false
}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, args []string) (atc.Worker, ifrit.Runner, error) {
	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	assetsDir, err := cmd.restoreVersionedAssets()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	btrfsToolsDir := filepath.Join(assetsDir, "btrfs")
	iptablesDir := filepath.Join(assetsDir, "iptables")

	err = os.Setenv(
		"PATH",
		strings.Join(
			[]string{
				btrfsToolsDir,
				filepath.Join(iptablesDir, "sbin"),
				filepath.Join(iptablesDir, "bin"),
				os.Getenv("PATH"),
			},
			string(os.PathListSeparator),
		),
	)
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

	cmd.Garden.Containers.Dir = guardiancmd.DirFlag(depotDir)

	cmd.Garden.Bin.Runc = filepath.Join(assetsDir, "bin", "runc")
	cmd.Garden.Bin.Dadoo = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "dadoo"))
	cmd.Garden.Bin.Init = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "init"))
	cmd.Garden.Bin.IODaemon = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "iodaemon"))
	cmd.Garden.Bin.Kawasaki = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "kawasaki"))
	cmd.Garden.Bin.NSTar = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "nstar"))
	cmd.Garden.Bin.Tar = guardiancmd.FileFlag(filepath.Join(assetsDir, "bin", "tar"))

	cmd.Garden.Network.AllowHostAccess = true

	worker := atc.Worker{
		Platform: "linux",
		Tags:     cmd.Tags,

		HTTPProxyURL:  cmd.HTTPProxy.String(),
		HTTPSProxyURL: cmd.HTTPSProxy.String(),
		NoProxy:       strings.Join(cmd.NoProxy, ","),
	}

	worker.ResourceTypes, err = cmd.extractResources(assetsDir)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	runner := guardiancmd.GuardianCommand(cmd.Garden)
	return worker, &runner, nil
}

func (cmd *WorkerCommand) restoreVersionedAssets() (string, error) {
	assetsDir := filepath.Join(cmd.WorkDir, Version)

	okMarker := filepath.Join(assetsDir, "ok")

	_, err := os.Stat(okMarker)
	if err == nil {
		return assetsDir, nil
	}

	err = bindata.RestoreAssets(assetsDir, "linux")
	if err != nil {
		return "", err
	}

	ok, err := os.Create(okMarker)
	if err != nil {
		return "", err
	}

	err = ok.Close()
	if err != nil {
		return "", err
	}

	return assetsDir, nil
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
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

	if fsStat.Type != btrfsFSType {
		filesystem := fs.New(logger.Session("fs"), volumesImage, volumesDir)

		err = filesystem.Create(fsStat.Blocks * uint64(fsStat.Bsize))
		if err != nil {
			logger.Error("falling-back-on-naive-driver", err)
			return cmd.naiveBaggageclaimRunner(logger)
		}
	}

	bc := &baggageclaimcmd.BaggageclaimCommand{
		BindIP:   baggageclaimcmd.IPFlag(cmd.Baggageclaim.BindIP.IP().String()),
		BindPort: cmd.Baggageclaim.BindPort,

		VolumesDir: baggageclaimcmd.DirFlag(volumesDir),

		Driver: "btrfs",

		ReapInterval: cmd.Baggageclaim.ReapInterval,

		Metrics: cmd.Metrics,
	}

	return bc.Runner(nil)
}

func (cmd *WorkerCommand) extractResources(assetsDir string) ([]atc.WorkerResourceType, error) {
	var resourceTypes []atc.WorkerResourceType

	binDir := filepath.Join(assetsDir, "bin")
	resourcesDir := filepath.Join(assetsDir, "resources")
	resourceImagesDir := filepath.Join(assetsDir, "resource-images")

	tarBin := filepath.Join(binDir, "tar")

	infos, err := ioutil.ReadDir(resourcesDir)
	if err != nil {
		return nil, err
	}

	for _, info := range infos {
		resourceType := info.Name()

		archive := filepath.Join(resourcesDir, resourceType, "rootfs.tar.gz")

		extractedDir := filepath.Join(resourceImagesDir, resourceType)

		imageDir := filepath.Join(extractedDir, "rootfs")
		okMarker := filepath.Join(extractedDir, "ok")

		var version string
		versionFile, err := os.Open(filepath.Join(resourcesDir, resourceType, "version"))
		if err != nil {
			return nil, err
		}

		_, err = fmt.Fscanf(versionFile, "%s", &version)
		if err != nil {
			return nil, err
		}

		defer versionFile.Close()

		_, err = os.Stat(okMarker)
		if err == os.ErrNotExist {
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

			ok, err := os.Create(okMarker)
			if err != nil {
				return nil, err
			}

			err = ok.Close()
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		resourceTypes = append(resourceTypes, atc.WorkerResourceType{
			Type:    resourceType,
			Image:   imageDir,
			Version: version,
		})
	}

	return resourceTypes, nil
}
