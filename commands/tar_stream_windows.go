// +build windows

package commands

import "io"

func tarStreamFrom(workDir string, paths []string) (io.ReadCloser, error) {
	return nativeTarGZStreamFrom(workDir, paths)
}
