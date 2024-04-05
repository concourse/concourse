// +build freebsd

package tarfs

import (
	"archive/tar"
	"syscall"

	"golang.org/x/sys/unix"
)

func mknodEntry(hdr *tar.Header, path string) error {
	mode := uint32(hdr.Mode & 07777)
	switch hdr.Typeflag {
	case tar.TypeBlock:
		mode |= unix.S_IFBLK
	case tar.TypeChar:
		mode |= unix.S_IFCHR
	case tar.TypeFifo:
		mode |= unix.S_IFIFO
	}

	return syscall.Mknod(path, mode, uint64(mkdev(hdr.Devmajor, hdr.Devminor)))
}

func mkdev(major, minor int64) uint32 {
	return uint32(((minor & 0xfff00) << 12) | ((major & 0xfff) << 8) | (minor & 0xff))
}
