package copy

import (
	"os/exec"
)

func Cp(followSymlinks bool, src, dest string) error {
	args := []string{"/e", "/nfl", "/ndl", "/mt"}
	if !followSymlinks {
		args = append(args, "/sl")
	}

	args = append(args, src, dest)

	return robocopy(args...)
}

func robocopy(args ...string) error {
	cmd := exec.Command("robocopy", args...)

	err := cmd.Run()
	if err != nil {
		// Robocopy returns a status code indicating what action occurred. 0 means nothing was copied,
		// 1 means that files were copied successfully. Google for additional error codes.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() > 1 {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}
