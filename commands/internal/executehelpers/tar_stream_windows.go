// +build windows

package executehelpers

import (
	"io"

	"github.com/concourse/go-archive/tgzfs"
)

func tarStreamFrom(workDir string, paths []string) (io.ReadCloser, error) {
	return nativeTarGZStreamFrom(workDir, paths)
}

func tarStreamTo(workDir string, stream io.Reader) error {
	return tgzfs.Extract(workDir, stream)
}
