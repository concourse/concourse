//go:build !windows

package copy

import (
	"errors"
	"os/exec"
)

func Cp(followSymlinks bool, src, dest string) error {
	cpFlags := "-a"
	if followSymlinks {
		cpFlags = "-Lr"
	}

	cmd := exec.Command("cp", cpFlags, src+"/.", dest)
	_, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return errors.Join(err, errors.New(string(exitErr.Stderr)))
		}
		return err
	}
	return nil
}
