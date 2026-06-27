package tarfs

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()

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

		err = extractEntry(root, hdr, tarReader, chown)
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
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	return extractEntry(root, header, input, chown)
}

const pathEscapesErr = "path escapes"

func extractEntry(root *os.Root, header *tar.Header, input io.Reader, chown bool) error {
	filePath := header.Name
	fileInfo := header.FileInfo()
	fileMode := fileInfo.Mode()

	err := root.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return err
	}

	switch header.Typeflag {
	case tar.TypeLink:
		header.Linkname = stripRoot(header.Linkname)

		err = root.Link(header.Linkname, filePath)
		if err != nil {
			// We only care about path-escape errors. All others are ignored and we fall through
			if linkErr, ok := errors.AsType[*os.LinkError](err); ok {
				if strings.Contains(linkErr.Error(), pathEscapesErr) {
					return BreakoutError{
						HeaderName: header.Name,
						LinkName:   header.Linkname,
					}
				}
			}
			return err
		}

	case tar.TypeSymlink:
		if !filepath.IsAbs(header.Linkname) {
			// Absolute symlinks are left as-is because in containerized
			// environments absolute paths won't escape the container. For
			// non-containerized environments, sorry, can't help ya! We do use
			// os.Root, so if someone tries to do an extraction attack by:
			// 1) Creating a symlink that points at an absolute path
			// 2) Create a file that would follow that symlink, writing outside
			//    the extraction path
			// os.Root won't allow that.
			// For symlinks pointing to relative paths, we detect if they try to
			// point to a path outside of the destination path and return a BreakoutErr
			_, err := root.Stat(filepath.Join(filepath.Dir(filePath), header.Linkname))
			if err != nil {
				// We only care about path-escape errors. All others are ignored and we fall through
				if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
					if strings.Contains(pathErr.Error(), pathEscapesErr) {
						return BreakoutError{
							HeaderName: header.Name,
							LinkName:   header.Linkname,
						}
					}
				}
			}
		}

		err = root.Symlink(filepath.FromSlash(header.Linkname), filePath)
		if err != nil {
			return err
		}

	case tar.TypeDir:
		err := root.MkdirAll(filePath, fileMode.Perm())
		if err != nil {
			return err
		}

	case tar.TypeReg, tar.TypeRegA:
		file, err := root.Create(filePath)
		if err != nil {
			return err
		}

		_, err = io.Copy(file, input)
		if err != nil {
			file.Close()
			return err
		}

		err = file.Close()
		if err != nil {
			return err
		}

	case tar.TypeBlock, tar.TypeChar, tar.TypeFifo:
		err := mknodEntry(header, filepath.Join(root.Name(), filePath))
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
		err = root.Lchown(filePath, header.Uid, header.Gid)
		if err != nil {
			return err
		}
	}

	// must be done after chown
	err = lchmod(root, header, filePath, fileMode)
	if err != nil {
		return err
	}

	// must be done after everything
	err = lchtimes(root, header, filePath)
	if err != nil {
		return err
	}

	return nil
}

func lchmod(root *os.Root, header *tar.Header, path string, fileMode os.FileMode) error {
	if header.Typeflag == tar.TypeLink {
		if fi, err := root.Lstat(header.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			return root.Chmod(path, fileMode)
		}
	} else if header.Typeflag != tar.TypeSymlink {
		return root.Chmod(path, fileMode)
	}

	return nil
}

func lchtimes(root *os.Root, header *tar.Header, path string) error {
	aTime := header.AccessTime
	mTime := header.ModTime
	if aTime.Before(mTime) {
		aTime = mTime
	}

	if header.Typeflag == tar.TypeLink {
		if fi, err := root.Lstat(header.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			return root.Chtimes(path, aTime, mTime)
		}
	} else if header.Typeflag != tar.TypeSymlink {
		return root.Chtimes(path, aTime, mTime)
	}

	return nil
}

func stripRoot(p string) string {
	// On Unix this part does nothing. On Windows it strips `C:`
	p = strings.TrimPrefix(p, filepath.VolumeName(p))
	// Removes all leading forward and backwards slashes
	p = strings.TrimLeft(p, `/\`)
	return p
}
