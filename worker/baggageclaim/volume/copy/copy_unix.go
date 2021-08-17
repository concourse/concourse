//+build !windows

package copy

import (
	"os/exec"
)

func Cp(followSymlinks bool, src, dest string) error {
	cpFlags := "-a"
	if followSymlinks {
		cpFlags = "-Lr"
	}

	return exec.Command("cp", cpFlags, src+"/.", dest).Run()
}
