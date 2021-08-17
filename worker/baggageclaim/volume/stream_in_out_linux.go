package volume

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/concourse/worker/baggageclaim/uidgid"
	"github.com/klauspost/compress/zstd"
)

func (streamer *tarZstdStreamer) In(tzstInput io.Reader, dest string, privileged bool) (bool, error) {
	tarCommand, dirFd, err := tarCmd(streamer.namespacer, privileged, dest, "-xf", "-")
	if err != nil {
		return false, err
	}

	defer dirFd.Close()

	zstdDecompressedStream, err := zstd.NewReader(tzstInput)
	if err != nil {
		return false, err
	}

	tarCommand.Stdin = zstdDecompressedStream
	tarCommand.Stdout = os.Stderr
	tarCommand.Stderr = os.Stderr

	err = tarCommand.Run()
	if err != nil {
		zstdDecompressedStream.Close()

		if _, ok := err.(*exec.ExitError); ok {
			return true, err
		}

		return false, err
	}

	zstdDecompressedStream.Close()

	return false, nil
}

func (streamer *tarZstdStreamer) Out(tzstOutput io.Writer, src string, privileged bool) error {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	var tarCommandPath, tarCommandDir string

	if fileInfo.IsDir() {
		tarCommandPath = "."
		tarCommandDir = src
	} else {
		tarCommandPath = filepath.Base(src)
		tarCommandDir = filepath.Dir(src)
	}

	tarCommand, dirFd, err := tarCmd(streamer.namespacer, privileged, tarCommandDir, "-cf", "-", tarCommandPath)
	if err != nil {
		return err
	}

	defer dirFd.Close()

	zstdCompressor, err := zstd.NewWriter(tzstOutput)
	if err != nil {
		return err
	}

	tarCommand.Stdout = zstdCompressor
	tarCommand.Stderr = os.Stderr

	err = tarCommand.Run()
	if err != nil {
		_ = zstdCompressor.Close()
		return err
	}

	err = zstdCompressor.Close()
	if err != nil {
		return err
	}

	return nil
}

func (streamer *tarGzipStreamer) In(tgzStream io.Reader, dest string, privileged bool) (bool, error) {
	tarCommand, dirFd, err := tarCmd(streamer.namespacer, privileged, dest, "-xz")
	if err != nil {
		return false, err
	}

	defer dirFd.Close()

	tarCommand.Stdin = tgzStream
	tarCommand.Stdout = os.Stderr
	tarCommand.Stderr = os.Stderr

	err = tarCommand.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return true, err
		}

		return false, err
	}

	return false, nil
}

func (streamer *tarGzipStreamer) Out(w io.Writer, src string, privileged bool) error {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	var tarCommandPath, tarCommandDir string

	if fileInfo.IsDir() {
		tarCommandPath = "."
		tarCommandDir = src
	} else {
		tarCommandPath = filepath.Base(src)
		tarCommandDir = filepath.Dir(src)
	}

	tarCommand, dirFd, err := tarCmd(streamer.namespacer, privileged, tarCommandDir, "-cz", tarCommandPath)
	if err != nil {
		return err
	}

	defer dirFd.Close()

	tarCommand.Stdout = w
	tarCommand.Stderr = os.Stderr

	err = tarCommand.Run()
	if err != nil {
		return err
	}

	return nil
}

func tarCmd(namespacer uidgid.Namespacer, privileged bool, dir string, args ...string) (*exec.Cmd, *os.File, error) {
	// 'tar' may run as MAX_UID in order to remap UIDs when streaming into an
	// unprivileged volume. this may cause permission issues when exec'ing as it
	// may not be able to even see the destination directory as non-root.
	//
	// so, open the directory while we're root, and pass it as a fd to the
	// process.
	dirFd, err := os.Open(dir)
	if err != nil {
		return nil, nil, err
	}

	tarCommand := exec.Command("tar", append([]string{"-C", "/dev/fd/3"}, args...)...)
	tarCommand.ExtraFiles = []*os.File{dirFd}

	if !privileged {
		namespacer.NamespaceCommand(tarCommand)
	}

	return tarCommand, dirFd, nil
}
