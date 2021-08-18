// +build !linux

package uidgid

import (
	"os/exec"
)

type noopMapper struct{}

func NewPrivilegedMapper() Mapper {
	return noopMapper{}
}

func NewUnprivilegedMapper() Mapper {
	return noopMapper{}
}

func (m noopMapper) Apply(cmd *exec.Cmd) {}

func (m noopMapper) Map(fromUid int, fromGid int) (int, int) {
	return fromUid, fromGid
}
