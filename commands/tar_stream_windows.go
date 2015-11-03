// +build windows

package commands

import (
	"io"

	"github.com/kr/tarutil"
)

func tarStreamFrom(workDir string, paths []string) (io.ReadCloser, error) {
	return nativeTarGZStreamFrom(workDir, paths)
}

func tarStreamTo(workDir string, stream io.Reader) error {
	return tarutil.ExtractAll(stream, workDir, tarutil.Chmod|tarutil.Chtimes|tarutil.Symlink)
}
