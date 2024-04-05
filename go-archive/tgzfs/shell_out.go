//+build !windows

package tgzfs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

func tarExtract(tarPath string, src io.Reader, dest string) error {
	err := os.MkdirAll(dest, 0755)
	if err != nil {
		return err
	}

	tarCmd := exec.Command(tarPath, "-pzxf", "-")
	tarCmd.Dir = dest
	tarCmd.Stdin = src

	// prevent ctrl+c and such from killing tar process
	tarCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	out, err := tarCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar extract failed (%s). output: %q", err, out)
	}

	return nil
}

func tarCompress(tarPath string, dest io.Writer, workDir string, paths ...string) error {
	out := new(bytes.Buffer)

	args := []string{"-czf", "-", "--null", "-T", "-"}
	if runtime.GOOS == "darwin" {
		args = append([]string{"--no-mac-metadata"}, args...)
	}
	tarCmd := exec.Command(tarPath, args...)
	tarCmd.Dir = workDir
	tarCmd.Stderr = out
	tarCmd.Stdout = dest

	tarCmd.Stdin = bytes.NewBufferString(strings.Join(paths, "\x00"))

	// prevent ctrl+c and such from killing tar process
	tarCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	err := tarCmd.Run()
	if err != nil {
		return fmt.Errorf("tar compress failed (%s). output: %q", err, out.String())
	}

	return nil
}
