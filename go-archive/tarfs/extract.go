package tarfs

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func Extract(src io.Reader, dest string) error {
	if runtime.GOOS != "windows" {
		tarPath, err := exec.LookPath("tar")
		if err == nil {
			return tarExtract(tarPath, src, dest)
		}
	}

	tarReader := tar.NewReader(src)

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

		err = ExtractEntry(hdr, dest, tarReader, chown)
		if err != nil {
			return err
		}
	}

	return nil
}

type BreakoutError struct {
	HeaderName string
	LinkName   string
}

func (err BreakoutError) Error() string {
	return fmt.Sprintf("entry '%s' links outside of target directory: %s", err.HeaderName, err.LinkName)
}

func ExtractEntry(header *tar.Header, dest string, input io.Reader, chown bool) error {
	filePath := filepath.Join(dest, header.Name)
	fileInfo := header.FileInfo()
	fileMode := fileInfo.Mode()

	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return err
	}

	switch header.Typeflag {
	case tar.TypeLink:
		targetPath := filepath.Join(dest, header.Linkname)

		if !strings.HasPrefix(targetPath, dest) {
			return BreakoutError{header.Name, header.Linkname}
		}

		err := os.Link(filepath.Join(dest, header.Linkname), filePath)
		if err != nil {
			return err
		}

	case tar.TypeSymlink:
		err := os.Symlink(header.Linkname, filePath)
		if err != nil {
			return err
		}

	case tar.TypeDir:
		err := os.MkdirAll(filePath, fileMode)
		if err != nil {
			return err
		}

	case tar.TypeReg, tar.TypeRegA:
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}

		_, err = io.Copy(file, input)
		if err != nil {
			return err
		}

		err = file.Close()
		if err != nil {
			return err
		}

	case tar.TypeBlock, tar.TypeChar, tar.TypeFifo:
		err := mknodEntry(header, filePath)
		if err != nil {
			return err
		}

	case tar.TypeXGlobalHeader:
		// skip
		return nil

	default:
		return fmt.Errorf("%s: unsupported entry type (%c)", header.Name, header.Typeflag)
	}

	if runtime.GOOS != "windows" && chown {
		err = os.Lchown(filePath, header.Uid, header.Gid)
		if err != nil {
			return err
		}
	}

	// must be done after chown
	err = lchmod(header, filePath, fileMode)
	if err != nil {
		return err
	}

	// must be done after everything
	err = lchtimes(header, filePath)
	if err != nil {
		return err
	}

	return nil
}

func lchmod(header *tar.Header, path string, fileMode os.FileMode) error {
	if header.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(header.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			return os.Chmod(path, fileMode)
		}
	} else if header.Typeflag != tar.TypeSymlink {
		return os.Chmod(path, fileMode)
	}

	return nil
}

func lchtimes(header *tar.Header, path string) error {
	aTime := header.AccessTime
	mTime := header.ModTime
	if aTime.Before(mTime) {
		aTime = mTime
	}

	if header.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(header.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			return os.Chtimes(path, aTime, mTime)
		}
	} else if header.Typeflag != tar.TypeSymlink {
		return os.Chtimes(path, aTime, mTime)
	}

	return nil
}
