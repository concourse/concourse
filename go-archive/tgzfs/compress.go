package tgzfs

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func Compress(dest io.Writer, workDir string, paths ...string) error {
	if runtime.GOOS != "windows" {
		if tarPath, err := exec.LookPath("tar"); err == nil {
			return tarCompress(tarPath, dest, workDir, paths...)
		}
	}

	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return err
	}

	gzWriter := gzip.NewWriter(dest)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, p := range paths {
		err := writePathToTar(tarWriter, absWorkDir, filepath.Join(absWorkDir, p))
		if err != nil {
			return err
		}
	}

	return nil
}

func writePathToTar(tw *tar.Writer, workDir string, srcPath string) error {
	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}

		return addTarFile(path, relative, tw)
	})
}

func addTarFile(path, name string, tw *tar.Writer) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	link := ""
	if fi.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}

	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return err
	}

	if fi.IsDir() && !os.IsPathSeparator(name[len(name)-1]) {
		name = name + "/"
	}

	if hdr.Typeflag == tar.TypeReg && name == "." {
		// archiving a single file
		hdr.Name = filepath.ToSlash(filepath.Base(path))
	} else {
		hdr.Name = filepath.ToSlash(name)
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	if hdr.Typeflag == tar.TypeReg {
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		defer file.Close()

		_, err = io.Copy(tw, file)
		if err != nil {
			return err
		}
	}

	return nil
}
