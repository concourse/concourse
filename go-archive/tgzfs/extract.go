package tgzfs

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/concourse/concourse/go-archive/tarfs"
)

func Extract(src io.Reader, dest string) error {
	if runtime.GOOS != "windows" {
		tarPath, err := exec.LookPath("tar")
		if err == nil {
			return tarExtract(tarPath, src, dest)
		}
	}

	gr, err := gzip.NewReader(src)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(gr)

	chown := os.Getuid() == 0

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if hdr.Name == "." {
			continue
		}

		err = tarfs.ExtractEntry(hdr, dest, tarReader, chown)
		if err != nil {
			return err
		}
	}

	return nil
}
