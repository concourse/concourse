package uidgid

import (
	"os"
	"os/exec"
	"syscall"
)

func (t *translator) getuidgid(info os.FileInfo) (int, int) {
	stat := info.Sys().(*syscall.Stat_t)
	return int(stat.Uid), int(stat.Gid)
}

func (t *translator) setuidgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
		Credential: &syscall.Credential{
			Uid: 0,
			Gid: 0,
		},
		GidMappingsEnableSetgroups: true,
	}

	t.mapper.Apply(cmd)
}
