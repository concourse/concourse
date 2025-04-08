//go:build !linux
// +build !linux

package volume

import (
	"io"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/go-archive/tarfs"
	"github.com/concourse/concourse/go-archive/tgzfs"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
)

func (streamer *tarGzipStreamer) In(stream io.Reader, dest string, privileged bool) (bool, error) {
	var err error
	if streamer.skipGzip {
		err = tarfs.Extract(stream, dest)
	} else {
		err = tgzfs.Extract(stream, dest)
	}
	if err != nil {
		return true, err
	}

	return false, nil
}

func (streamer *tarGzipStreamer) Out(w io.Writer, src string, privileged bool) error {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	var tarDir, tarPath string

	if fileInfo.IsDir() {
		tarDir = src
		tarPath = "."
	} else {
		tarDir = filepath.Dir(src)
		tarPath = filepath.Base(src)
	}

	if streamer.skipGzip {
		return tarfs.Compress(w, tarDir, tarPath)
	} else {
		return tgzfs.Compress(w, tarDir, tarPath)
	}
}

func (streamer *tarZstdStreamer) In(stream io.Reader, dest string, privileged bool) (bool, error) {
	zstdStreamReader, err := zstd.NewReader(stream)
	if err != nil {
		return true, err
	}

	err = tarfs.Extract(zstdStreamReader, dest)
	if err != nil {
		zstdStreamReader.Close()
		return true, err
	}

	zstdStreamReader.Close()

	return false, nil
}

func (streamer *tarZstdStreamer) Out(w io.Writer, src string, privileged bool) error {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	var tarDir, tarPath string

	if fileInfo.IsDir() {
		tarDir = src
		tarPath = "."
	} else {
		tarDir = filepath.Dir(src)
		tarPath = filepath.Base(src)
	}

	zstdStreamWriter, err := zstd.NewWriter(w)
	if err != nil {
		return err
	}

	err = tarfs.Compress(zstdStreamWriter, tarDir, tarPath)
	if err != nil {
		zstdStreamWriter.Close()
		return err
	}

	err = zstdStreamWriter.Close()
	if err != nil {
		return err
	}

	return nil
}

func (streamer *tarS2Streamer) In(stream io.Reader, dest string, privileged bool) (bool, error) {
	s2StreamReader := s2.NewReader(stream)

	err := tarfs.Extract(s2StreamReader, dest)
	if err != nil {
		return true, err
	}

	return false, nil
}

func (streamer *tarS2Streamer) Out(w io.Writer, src string, privileged bool) error {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	var tarDir, tarPath string

	if fileInfo.IsDir() {
		tarDir = src
		tarPath = "."
	} else {
		tarDir = filepath.Dir(src)
		tarPath = filepath.Base(src)
	}

	s2StreamWriter := s2.NewWriter(w)

	err = tarfs.Compress(s2StreamWriter, tarDir, tarPath)
	if err != nil {
		s2StreamWriter.Close()
		return err
	}

	err = s2StreamWriter.Close()
	if err != nil {
		return err
	}

	return nil
}
