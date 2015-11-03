// +build windows

package commands

import (
	"compress/gzip"
	"io"

	"github.com/kr/tarutil"
)

func tarStreamFrom(workDir string, paths []string) (io.ReadCloser, error) {
	return nativeTarGZStreamFrom(workDir, paths)
}

func tarStreamTo(workDir string, stream io.Reader) error {
	gr, err := gzip.NewReader(stream)
	if err != nil {
		return err
	}

	return tarutil.ExtractAll(gr, workDir, tarutil.Chmod|tarutil.Chtimes|tarutil.Symlink)
}
