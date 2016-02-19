// +build !windows

package executehelpers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/kr/tarutil"
)

func tarStreamFrom(workDir string, paths []string) (io.ReadCloser, error) {
	var archive io.ReadCloser

	if tarPath, err := exec.LookPath("tar"); err == nil {
		tarCmd := exec.Command(tarPath, "-czf", "-", "--null", "-T", "-")
		tarCmd.Dir = workDir
		tarCmd.Stderr = os.Stderr

		tarCmd.Stdin = bytes.NewBufferString(strings.Join(paths, "\x00"))

		tarCmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		archive, err = tarCmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("could not create tar pipe: %s", err)
		}

		err = tarCmd.Start()
		if err != nil {
			return nil, fmt.Errorf("could not run tar: %s", err)
		}
	} else {
		return nativeTarGZStreamFrom(workDir, paths)
	}

	return archive, nil
}

func tarStreamTo(workDir string, stream io.Reader) error {
	if tarPath, err := exec.LookPath("tar"); err == nil {
		tarCmd := exec.Command(tarPath, "-xzf", "-")
		tarCmd.Dir = workDir
		tarCmd.Stderr = os.Stderr

		tarCmd.Stdin = stream

		tarCmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		return tarCmd.Run()
	}

	gr, err := gzip.NewReader(stream)
	if err != nil {
		return err
	}

	return tarutil.ExtractAll(gr, workDir, tarutil.Chmod|tarutil.Chtimes|tarutil.Symlink)
}
