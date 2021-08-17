package driver

import (
	"bytes"
	"os/exec"
	"syscall"

	"github.com/concourse/baggageclaim/volume"
)

func (driver *NaiveDriver) CreateCopyOnWriteLayer(
	childVol volume.FilesystemInitVolume,
	parentVol volume.FilesystemLiveVolume,
) error {
	_, err := robocopy("/e", "/nfl", "/ndl", parentVol.DataPath(), childVol.DataPath())
	return err
}

func robocopy(args ...string) (string, error) {
	stdout := &bytes.Buffer{}

	cmd := exec.Command("robocopy", args...)
	cmd.Stdout = stdout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	// Robocopy returns a status code indicating what action occurred. 0 means nothing was copied,
	// 1 means that files were copied successfully. Google for additional error codes.
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() > 1 {
					return "", err
				}
			}
		} else {
			return "", err
		}
	}

	return stdout.String(), nil
}
