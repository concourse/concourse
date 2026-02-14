package driver

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

var _ volume.Driver = (*BtrFSDriver)(nil)

type BtrFSDriver struct {
	logger                lager.Logger
	btrfsBin              string
	supportsRecursiveFlag bool
}

func NewBtrFSDriver(
	logger lager.Logger,
	btrfsBin string,
) *BtrFSDriver {
	driver := &BtrFSDriver{
		logger:   logger.Session("btrfs-driver"),
		btrfsBin: btrfsBin,
	}

	driver.supportsRecursiveFlag = driver.checkRecursiveFlagSupport()

	return driver
}

func (driver *BtrFSDriver) CreateVolume(vol volume.FilesystemInitVolume) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "create", vol.DataPath())
	return err
}

func (driver *BtrFSDriver) DestroyVolume(vol volume.FilesystemVolume) error {
	if driver.supportsRecursiveFlag {
		driver.logger.Debug("using-recursive-flag", lager.Data{"path": vol.DataPath()})
		_, _, err := driver.run(driver.btrfsBin, "subvolume", "delete", "--recursive", vol.DataPath())
		return err
	}

	// Fall back to manual recursive deletion
	driver.logger.Debug("using-manual-recursion", lager.Data{"path": vol.DataPath()})

	volumePathsToDelete := []string{}

	findSubvolumes := func(p string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !f.IsDir() {
			return nil
		}

		isSub, err := isSubvolume(p)
		if err != nil {
			return fmt.Errorf("failed to check if %s is a subvolume: %s", p, err)
		}

		if isSub {
			volumePathsToDelete = append(volumePathsToDelete, p)
		}

		return nil
	}

	if err := filepath.Walk(vol.DataPath(), findSubvolumes); err != nil {
		return fmt.Errorf("recursively walking subvolumes for %s failed: %v", vol.DataPath(), err)
	}

	for i := len(volumePathsToDelete) - 1; i >= 0; i-- {
		_, _, err := driver.run(driver.btrfsBin, "subvolume", "delete", volumePathsToDelete[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (driver *BtrFSDriver) CreateCopyOnWriteLayer(
	childVol volume.FilesystemInitVolume,
	parentVol volume.FilesystemLiveVolume,
) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "snapshot", parentVol.DataPath(), childVol.DataPath())
	return err
}

func (driver *BtrFSDriver) checkRecursiveFlagSupport() bool {
	cmd := exec.Command(driver.btrfsBin, "subvolume", "delete", "--help")
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout

	_ = cmd.Run()

	supportsFlag := strings.Contains(stdout.String(), "--recursive")

	driver.logger.Info("btrfs-recursive-flag-check", lager.Data{
		"supports-recursive-flag": supportsFlag,
	})

	return supportsFlag
}

func (driver *BtrFSDriver) run(command string, args ...string) (string, string, error) {
	cmd := exec.Command(command, args...)

	logger := driver.logger.Session("run-command", lager.Data{
		"command": command,
		"args":    args,
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	loggerData := lager.Data{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		logger.Error("failed", err, loggerData)
		return "", "", err
	}

	logger.Debug("ran", loggerData)

	return stdout.String(), stderr.String(), nil
}

func (driver *BtrFSDriver) Recover(volume.Filesystem) error {
	// nothing to do
	return nil
}

func (driver *BtrFSDriver) RemoveOrphanedResources(_ map[string]struct{}) error {
	// nothing to do. btrfs volumes live under the managed volume/ directory
	return nil
}
