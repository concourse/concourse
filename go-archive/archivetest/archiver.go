package archivetest

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"time"
)

type Archive []ArchiveFile

type ArchiveFile struct {
	Name string
	Body string
	Mode int64
	Dir  bool
	Link string

	ModTime    time.Time
	AccessTime time.Time
}

func (files Archive) TarGZStream() (io.Reader, error) {
	buf := new(bytes.Buffer)

	gw := gzip.NewWriter(buf)

	err := files.WriteTar(gw)
	if err != nil {
		return nil, err
	}

	err = gw.Close()
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (files Archive) TarStream() (io.Reader, error) {
	buf := new(bytes.Buffer)

	err := files.WriteTar(buf)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (files Archive) ZipFile(tmpDir string) (string, error) {
	zipFile, err := ioutil.TempFile("", "archivetest-zip")
	if err != nil {
		return "", err
	}

	defer zipFile.Close()

	err = files.WriteZip(zipFile)
	if err != nil {
		return "", err
	}

	return zipFile.Name(), nil
}

func (files Archive) WriteTar(writer io.Writer) error {
	w := tar.NewWriter(writer)

	for _, file := range files {
		var header *tar.Header

		mode := file.Mode
		if mode == 0 {
			mode = 0777
		}

		if file.Dir {
			header = &tar.Header{
				Name:     file.Name,
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}
		} else if file.Link != "" {
			header = &tar.Header{
				Name:     file.Name,
				Typeflag: tar.TypeSymlink,
				Linkname: file.Link,
				Mode:     file.Mode,
			}
		} else {
			header = &tar.Header{
				Typeflag:   tar.TypeReg,
				Name:       file.Name,
				Mode:       mode,
				Size:       int64(len(file.Body)),
				ModTime:    file.ModTime,
				AccessTime: file.AccessTime,
			}
		}

		err := w.WriteHeader(header)
		if err != nil {
			return err
		}

		_, err = w.Write([]byte(file.Body))
		if err != nil {
			return err
		}
	}

	err := w.Close()
	if err != nil {
		return err
	}

	return nil
}

func (files Archive) WriteZip(writer io.Writer) error {
	w := zip.NewWriter(writer)

	for _, file := range files {
		header := &zip.FileHeader{
			Name: file.Name,
		}

		mode := file.Mode
		if mode == 0 {
			mode = 0777
		}

		if file.Link != "" {
			header.SetMode(os.FileMode(mode) | os.ModeSymlink)
		} else {
			header.SetMode(os.FileMode(mode))
		}

		if !file.ModTime.IsZero() {
			header.SetModTime(file.ModTime)
		}

		f, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		if file.Link != "" {
			_, err = f.Write([]byte(file.Link))
		} else {
			_, err = f.Write([]byte(file.Body))
		}
		if err != nil {
			return err
		}
	}

	return w.Close()
}
