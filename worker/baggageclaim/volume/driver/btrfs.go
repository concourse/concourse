package driver

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

type BtrFSDriver struct {
	logger   lager.Logger
	btrfsBin string
}

func NewBtrFSDriver(
	logger lager.Logger,
	btrfsBin string,
) *BtrFSDriver {
	return &BtrFSDriver{
		logger:   logger,
		btrfsBin: btrfsBin,
	}
}

func (driver *BtrFSDriver) CreateVolume(vol volume.FilesystemInitVolume) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "create", vol.DataPath())
	if err != nil {
		return err
	}

	return nil
}

func (driver *BtrFSDriver) DestroyVolume(vol volume.FilesystemVolume) error {
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
