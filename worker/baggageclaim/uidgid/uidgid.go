// +build !linux

package uidgid

import (
	"os"
	"os/exec"
)

func (t *translator) getuidgid(info os.FileInfo) (int, int) {
	panic("not supported")
}

func (t *translator) setuidgid(cmd *exec.Cmd) {
	panic("not supported")
}

func newMappings(maxID int) Mapper {
	panic("not supported")
}
