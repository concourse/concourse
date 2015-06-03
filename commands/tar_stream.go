// +build !windows

package commands

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

func tarStreamFrom(workDir string, paths []string) (io.ReadCloser, error) {
	var archive io.ReadCloser

	if tarPath, err := exec.LookPath("tar"); err == nil {
		tarCmd := exec.Command(tarPath, []string{"-czf", "-", "--null", "-T", "-"}...)
		tarCmd.Dir = workDir
		tarCmd.Stderr = os.Stderr

		tarCmd.Stdin = bytes.NewBufferString(strings.Join(paths, "\x00"))

		archive, err = tarCmd.StdoutPipe()
		if err != nil {
			log.Fatalln("could not create tar pipe:", err)
		}

		err = tarCmd.Start()
		if err != nil {
			log.Fatalln("could not run tar:", err)
		}
	} else {
		return nativeTarGZStreamFrom(workDir, paths)
	}

	return archive, nil
}
