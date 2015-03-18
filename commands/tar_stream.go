// +build !windows

package commands

import (
	"io"
	"log"
	"os"
	"os/exec"
)

func tarStreamFrom(workDir string, paths []string) (io.ReadCloser, error) {
	var archive io.ReadCloser

	if tarPath, err := exec.LookPath("tar"); err == nil {
		tarCmd := exec.Command(tarPath, append([]string{"-czf", "-"}, paths...)...)
		tarCmd.Dir = workDir
		tarCmd.Stderr = os.Stderr

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
