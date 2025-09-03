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
	for _, mapping := range idMap {
		// Check if the fromID is within this mapping range
		startID := mapping.ContainerID
		endID := mapping.ContainerID + mapping.Size - 1

		if fromID >= startID && fromID <= endID {
			// Calculate the offset within the range
			offset := fromID - mapping.ContainerID

			// Apply the offset to the host ID base
			return mapping.HostID + offset
		}
	}

	// If no mapping found, return the original ID
	return fromID
}

func (m uidGidMapper) Map(fromUid int, fromGid int) (int, int) {
	return findMapping(m.uids, fromUid), findMapping(m.gids, fromGid)
}
