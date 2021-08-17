package driver

import "syscall"

const btrfsVolumeIno = 256

func isSubvolume(p string) (bool, error) {
	var bufStat syscall.Stat_t
	if err := syscall.Lstat(p, &bufStat); err != nil {
		return false, err
	}

	return bufStat.Ino == btrfsVolumeIno, nil
}
