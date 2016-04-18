package main

import (
	"bufio"
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

	err = bindata.RestoreAssets(cmd.WorkDir, "linux")
	if err != nil {
		return atc.Worker{}, nil, err
	}

	linux := filepath.Join(cmd.WorkDir, "linux")

	btrfsToolsDir := filepath.Join(linux, "btrfs")
	iptablesDir := filepath.Join(linux, "iptables")

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

	busyboxDir, err := cmd.extractBusybox(linux)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	depotDir := filepath.Join(linux, "depot")

	// must be readable by other users so unprivileged containers can run their
	// own `initc' process
	err = os.MkdirAll(depotDir, 0755)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	cmd.Garden.Server.BindIP = guardiancmd.IPFlag(cmd.BindIP)

	cmd.Garden.Containers.Dir = guardiancmd.DirFlag(depotDir)
	cmd.Garden.Containers.DefaultRootFSDir = guardiancmd.DirFlag(busyboxDir)

	cmd.Garden.Bin.Runc = filepath.Join(linux, "bin", "runc")
	cmd.Garden.Bin.Dadoo = guardiancmd.FileFlag(filepath.Join(linux, "bin", "dadoo"))
	cmd.Garden.Bin.Init = guardiancmd.FileFlag(filepath.Join(linux, "bin", "init"))
	cmd.Garden.Bin.IODaemon = guardiancmd.FileFlag(filepath.Join(linux, "bin", "iodaemon"))
	cmd.Garden.Bin.Kawasaki = guardiancmd.FileFlag(filepath.Join(linux, "bin", "kawasaki"))
	cmd.Garden.Bin.NSTar = guardiancmd.FileFlag(filepath.Join(linux, "bin", "nstar"))
	cmd.Garden.Bin.Tar = guardiancmd.FileFlag(filepath.Join(linux, "bin", "tar"))

	cmd.Garden.Network.AllowHostAccess = true

	worker := atc.Worker{
		Platform: "linux",
		Tags:     cmd.Tags,

		HTTPProxyURL:  cmd.HTTPProxy.String(),
		HTTPSProxyURL: cmd.HTTPSProxy.String(),
		NoProxy:       strings.Join(cmd.NoProxy, ","),
	}

	worker.ResourceTypes, err = cmd.extractResources(linux)

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	runner := guardiancmd.GuardianCommand(cmd.Garden)
	return worker, &runner, nil
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	supportsBtrfs, err := cmd.supportsBtrfs()
	if err != nil {
		logger.Error("failed-to-check-for-btrfs-support", err)
		return nil, err
	}

	if !supportsBtrfs {
		logger.Info("using-native-volume-driver", nil)
		return cmd.naiveBaggageclaimRunner(logger)
	}

	volumesImage := filepath.Join(cmd.WorkDir, "volumes.img")
	volumesDir := filepath.Join(cmd.WorkDir, "volumes")

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
		filesystem := fs.New(logger.Session("fs"), volumesImage, volumesDir)

		err = filesystem.Create(fsStat.Blocks * uint64(fsStat.Bsize))
		if err != nil {
			return nil, fmt.Errorf("failed to set up volumes filesystem: %s", err)
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

func (cmd *WorkerCommand) supportsBtrfs() (bool, error) {
	fs, err := os.Open("/proc/filesystems")
	if err != nil {
		return false, err
	}

	defer fs.Close()

	scanner := bufio.NewScanner(fs)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "btrfs") {
			return true, nil
		}
	}

	return false, nil
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
