package copy

import (
	"errors"
	"os/exec"
)

func Cp(followSymlinks bool, src, dest string) error {
	args := []string{
		"/e",   // copy subdir's even if they're empty
		"/nfl", // don't log ever file copied
		"/ndl", // don't log ever dir copied
		"/mt",  // do multi-thread copying, defaults to 8 threads

		// retrying has a default value of 1 million with 30s waits, which is
		// 347 days of retrying. Let's override that.
		"/r:5", // retry any failed copy up to 5 times
		"/w:5", // wait 5 seconds between each retry
	}

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
				return errors.Join(err, errors.New(string(exitErr.Stderr)))
			}
		} else {
			return err
		}
	}

	return nil
}
