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
	var writer io.WriteCloser

	if tarPath, err := exec.LookPath("tar"); err == nil {
		tarCmd := exec.Command(tarPath, []string{"-czf", "-", "-T", "-"}...)
		tarCmd.Dir = workDir
		tarCmd.Stderr = os.Stderr

		archive, err = tarCmd.StdoutPipe()
		if err != nil {
			log.Fatalln("could not create tar pipe:", err)
		}

		writer, err = tarCmd.StdinPipe()
		if err != nil {
			log.Fatalln("could not create tar stdin pipe:", err)
		}

		err = tarCmd.Start()
		if err != nil {
			log.Fatalln("could not run tar:", err)
		}

		go func() {
			for _, s := range paths {
				io.WriteString(writer, s+"\n")
			}
			writer.Close()
		}()
	} else {
		return nativeTarGZStreamFrom(workDir, paths)
	}

	return archive, nil
}
