package runtimetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing/fstest"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/worker/baggageclaim"
)

type VolumeContent fstest.MapFS

type Volume struct {
	Content VolumeContent

	VolumeHandle              string
	ResourceCacheInitialized  bool
	ResourceCacheStreamedFrom string
	TaskCacheInitialized      bool
	DBVolume_                 *dbfakes.FakeCreatedVolume
}

func NewVolume(handle string) *Volume {
	dbVolume := new(dbfakes.FakeCreatedVolume)
	dbVolume.HandleReturns(handle)
	return &Volume{
		VolumeHandle: handle,
		DBVolume_:    dbVolume,
		Content:      VolumeContent{},
	}
}

func (v Volume) WithContent(content VolumeContent) *Volume {
	v.Content = content
	return &v
}

func (v Volume) Handle() string {
	return v.VolumeHandle
}

func (v Volume) StreamIn(ctx context.Context, path string, compression compression.Compression, reader io.Reader) error {
	return v.Content.StreamIn(ctx, path, compression.Encoding(), reader)
}

func (v Volume) StreamOut(ctx context.Context, path string, compression compression.Compression) (io.ReadCloser, error) {
	return v.Content.StreamOut(ctx, path, compression.Encoding())
}

func (v *Volume) InitializeResourceCache(_ lager.Logger, _ db.ResourceCache) error {
	v.ResourceCacheInitialized = true
	return nil
}

func (v *Volume) InitializeStreamedResourceCache(_ lager.Logger, _ db.ResourceCache, workerName string) error {
	v.ResourceCacheInitialized = true
	v.ResourceCacheStreamedFrom = workerName
	return nil
}

func (v *Volume) InitializeTaskCache(_ lager.Logger, _ int, _, _ string, _ bool) error {
	v.TaskCacheInitialized = true
	return nil
}

func (v Volume) DBVolume() db.CreatedVolume {
	return v.DBVolume_
}

func (vc VolumeContent) StreamIn(ctx context.Context, path string, encoding baggageclaim.Encoding, tarStream io.Reader) error {
	if encoding != baggageclaim.GzipEncoding {
		return errors.New("only gzip is supported for runtimetest.VolumeContent")
	}

	gzipReader, err := gzip.NewReader(tarStream)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			filePath := filepath.Join(removeLeadingSlash(path), header.Name)
			fileData := new(bytes.Buffer)
			if _, err := io.Copy(fileData, tarReader); err != nil {
				return err
			}
			vc[filePath] = &fstest.MapFile{Data: fileData.Bytes()}
		default:
			panic(fmt.Sprintf("unexpected tar type: %v", header.Typeflag))
		}
	}
}

func (vc VolumeContent) StreamOut(ctx context.Context, path string, encoding baggageclaim.Encoding) (io.ReadCloser, error) {
	if encoding != baggageclaim.GzipEncoding {
		return nil, errors.New("only gzip is supported for runtimetest.VolumeContent")
	}

	buf := new(bytes.Buffer)

	gzipWriter := gzip.NewWriter(buf)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err := fs.WalkDir(fstest.MapFS(vc), removeLeadingSlash(path), func(filePath string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		info, err := dirEntry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, filePath)
		if err != nil {
			return err
		}
		header.Name = filePath
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		file, err := fstest.MapFS(vc).Open(filePath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tarWriter, file); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := tarWriter.Flush(); err != nil {
		return nil, err
	}
	return io.NopCloser(buf), nil
}

func removeLeadingSlash(path string) string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "."
	}
	return path
}
