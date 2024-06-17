// +build windows

package tarfs

import "archive/tar"

func mknodEntry(hdr *tar.Header, path string) error {
	return nil
}
