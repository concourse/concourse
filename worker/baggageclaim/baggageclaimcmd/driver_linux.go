package baggageclaimcmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim/fs"
	"github.com/concourse/concourse/worker/baggageclaim/kernel"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"
)

const btrfsFSType = 0x9123683e

func (cmd *BaggageclaimCommand) driver(logger lager.Logger) (volume.Driver, error) {
	var volMountInfo syscall.Statfs_t
	err := syscall.Statfs(cmd.VolumesDir.Path(), &volMountInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to stat volumes filesystem: %s", err)
	}

	kernelSupportsOverlay, err := kernel.CheckKernelVersion(4, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to check kernel version: %s", err)
	}

	// we don't care about the error here
	_ = exec.Command("modprobe", "btrfs").Run()

	supportsBtrfs, err := supportsFilesystem("btrfs")
	if err != nil {
		return nil, fmt.Errorf("failed to detect if btrfs is supported: %s", err)
	}

	_, err = exec.LookPath(cmd.BtrfsBin)
	if err != nil {
		supportsBtrfs = false
	}

	_, err = exec.LookPath(cmd.MkfsBin)
	if err != nil {
		supportsBtrfs = false
	}

	if cmd.Driver == "detect" {
		if kernelSupportsOverlay {
			cmd.Driver = "overlay"
		} else if supportsBtrfs {
			cmd.Driver = "btrfs"
		} else {
			cmd.Driver = "naive"
		}
	}

	volumesDir := cmd.VolumesDir.Path()
	btrfsImg := volumesDir + ".img"
	btrfsFS := fs.New(logger.Session("fs"), btrfsImg, volumesDir, cmd.MkfsBin)

	if cmd.Driver == "btrfs" && !isMountBtrfs(volMountInfo) {
		diskSize := volMountInfo.Blocks * uint64(volMountInfo.Bsize)
		mountSize := diskSize - (10 * 1024 * 1024 * 1024)
		if int64(mountSize) < 0 {
			mountSize = diskSize
		}

		err = btrfsFS.Create(mountSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create btrfs filesystem: %s", err)
		}
	}

	if cmd.Driver == "overlay" {
		if !kernelSupportsOverlay {
			return nil, errors.New("overlay driver requires kernel version >= 4.0.0")
		}
		// Clean up existing btrfs mount so we don't make overlay mounts inside
		// a btrfs mount. Stuff gets flakey otherwise
		if isMountBtrfs(volMountInfo) {
			err = btrfsFS.Delete()
			if err != nil {
				return nil, fmt.Errorf("failed to delete existing btrfs filesystem at %s: %s", cmd.VolumesDir.Path(), err)
			}
		}
	}

	logger.Info("using-driver", lager.Data{"driver": cmd.Driver})

	var d volume.Driver
	switch cmd.Driver {
	case "overlay":
		d = driver.NewOverlayDriver(cmd.OverlaysDir)
	case "btrfs":
		d = driver.NewBtrFSDriver(logger.Session("driver"), cmd.BtrfsBin)
	case "naive":
		d = &driver.NaiveDriver{}
	default:
		return nil, fmt.Errorf("unknown driver: %s", cmd.Driver)
	}

	return d, nil
}

func supportsFilesystem(fs string) (bool, error) {
	filesystems, err := os.Open("/proc/filesystems")
	if err != nil {
		return false, err
	}

	defer filesystems.Close()

	fsio := bufio.NewReader(filesystems)

	fsMatch := []byte(fs)

	for {
		line, _, err := fsio.ReadLine()
		if err != nil {
			if err == io.EOF {
				return false, nil
			}

			return false, err
		}

		if bytes.Contains(line, fsMatch) {
			return true, nil
		}
	}

	return false, nil
}

func isMountBtrfs(volMountInfo syscall.Statfs_t) bool {
	return uint32(volMountInfo.Type) == btrfsFSType
}
