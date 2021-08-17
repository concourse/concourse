package uidgid

import (
	"os/exec"
	"syscall"
)

type uidGidMapper struct {
	uids []syscall.SysProcIDMap
	gids []syscall.SysProcIDMap
}

func NewPrivilegedMapper() Mapper {
	maxID := min(MustGetMaxValidUID(), MustGetMaxValidGID())

	return uidGidMapper{
		uids: []syscall.SysProcIDMap{
			{ContainerID: maxID, HostID: 0, Size: 1},
			{ContainerID: 1, HostID: 1, Size: maxID - 1},
		},
		gids: []syscall.SysProcIDMap{
			{ContainerID: maxID, HostID: 0, Size: 1},
			{ContainerID: 1, HostID: 1, Size: maxID - 1},
		},
	}
}

func NewUnprivilegedMapper() Mapper {
	maxID := min(MustGetMaxValidUID(), MustGetMaxValidGID())

	return uidGidMapper{
		uids: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: maxID, Size: 1},
			{ContainerID: 1, HostID: 1, Size: maxID - 1},
		},
		gids: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: maxID, Size: 1},
			{ContainerID: 1, HostID: 1, Size: maxID - 1},
		},
	}
}

func (m uidGidMapper) Apply(cmd *exec.Cmd) {
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: uint32(m.uids[0].ContainerID),
		Gid: uint32(m.gids[0].ContainerID),
	}

	cmd.SysProcAttr.UidMappings = m.uids
	cmd.SysProcAttr.GidMappings = m.gids
}

func findMapping(idMap []syscall.SysProcIDMap, fromID int) int {
	for _, id := range idMap {
		if id.Size != 1 {
			continue
		}

		if id.ContainerID == fromID {
			return id.HostID
		}
	}

	return fromID
}

func (m uidGidMapper) Map(fromUid int, fromGid int) (int, int) {
	return findMapping(m.uids, fromUid), findMapping(m.gids, fromGid)
}
