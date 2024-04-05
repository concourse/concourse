package tarfs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func tarExtract(tarPath string, src io.Reader, dest string) error {
	err := os.MkdirAll(dest, 0755)
	if err != nil {
		return err
	}

	tarCmd := exec.Command(tarPath, "pzxf", "-", "-C", dest)
	tarCmd.Stdin = src

	out, err := tarCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar extract failed (%s). output: %q", err, out)
	}

	return nil
}

func tarCompress(tarPath string, dest io.Writer, workDir string, paths ...string) error {
	out := new(bytes.Buffer)

	tarCmd := exec.Command(tarPath, "-czf", "-", "--null", "-T", "-")
	tarCmd.Dir = workDir
	tarCmd.Stderr = out
	tarCmd.Stdout = dest

	tarCmd.Stdin = bytes.NewBufferString(strings.Join(paths, "\x00"))

	err := tarCmd.Run()
	if err != nil {
		return fmt.Errorf("tar compress failed (%s). output: %q", err, out.String())
	}

	return nil
}
