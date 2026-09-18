package zipfs

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/klauspost/compress/zip"
)

func Extract(srcFile string, dest string) error {
	path, err := exec.LookPath("unzip")
	if err == nil {
		err = os.MkdirAll(dest, 0755)
		if err != nil {
			return err
		}

		unzipCmd := exec.Command(path, srcFile)
		unzipCmd.Dir = dest

		return unzipCmd.Run()
	} else {
		files, err := zip.OpenReader(srcFile)
		if err != nil {
			return err
		}

		defer files.Close()
		root, err := os.OpenRoot(dest)
		if err != nil {
			return err
		}
		defer root.Close()

		for _, file := range files.File {
			err = func() error {
				readCloser, err := file.Open()
				if err != nil {
					return err
				}
				defer readCloser.Close()

				return extractZipArchiveFile(root, file, readCloser)
			}()

			if err != nil {
				return err
			}
		}

		return nil
	}
}

func extractZipArchiveFile(root *os.Root, file *zip.File, input io.Reader) error {
	filePath := file.Name
	fileInfo := file.FileInfo()

	if fileInfo.IsDir() {
		err := root.MkdirAll(filePath, fileInfo.Mode().Perm())
		if err != nil {
			return err
		}
	} else {
		err := root.MkdirAll(filepath.Dir(filePath), 0755)
		if err != nil {
			return err
		}

		if fileInfo.Mode()&os.ModeSymlink != 0 {
			linkName, err := io.ReadAll(input)
			if err != nil {
				return err
			}
			return root.Symlink(string(linkName), filePath)
		}

		fileCopy, err := root.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileInfo.Mode())
		if err != nil {
			return err
		}
		defer fileCopy.Close()

		_, err = io.Copy(fileCopy, input)
		if err != nil {
			return err
		}
	}

	return nil
}
