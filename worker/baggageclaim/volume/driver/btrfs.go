package driver

import (
	"bytes"
	"os/exec"

	"code.cloudfoundry.org/lager/v3"
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
	return err
}

func (driver *BtrFSDriver) DestroyVolume(vol volume.FilesystemVolume) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "delete", "-R", vol.DataPath())
	return err
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
