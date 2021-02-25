package gardenruntimetest

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
	"testing/fstest"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
)

type Baggageclaim struct {
	Volumes []*Volume
}

func (b Baggageclaim) FindVolume(handle string) (*Volume, int, bool) {
	for i, v := range b.Volumes {
		if v.handle == handle {
			return v, i, true
		}
	}
	return nil, 0, false
}

func (b Baggageclaim) FilteredVolumes(pred func(*Volume) bool) []*Volume {
	var filtered []*Volume
	for _, v := range b.Volumes {
		if pred(v) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func (b *Baggageclaim) AddVolume(volume *Volume) *Volume {
	_, i, ok := b.FindVolume(volume.handle)
	if ok {
		b.Volumes[i] = volume
		return volume
	}
	b.Volumes = append(b.Volumes, volume)
	return volume
}

func (b *Baggageclaim) CreateVolume(_ lager.Logger, handle string, spec baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
	volume := b.AddVolume(NewVolume(handle).WithSpec(spec))
	return volume, nil
}

func (b Baggageclaim) ListVolumes(_ lager.Logger, filter baggageclaim.VolumeProperties) (baggageclaim.Volumes, error) {
	filteredVolumes := b.FilteredVolumes(func(v *Volume) bool {
		return matchesFilter(v.Spec.Properties, filter)
	})
	bcVolumes := make([]baggageclaim.Volume, len(filteredVolumes))
	for i, vol := range filteredVolumes {
		bcVolumes[i] = vol
	}
	return bcVolumes, nil
}

func (b Baggageclaim) LookupVolume(_ lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
	v, _, ok := b.FindVolume(handle)
	return v, ok, nil
}

func (b Baggageclaim) DestroyVolumes(logger lager.Logger, handles []string) error {
	for _, handle := range handles {
		b.DestroyVolume(logger, handle)
	}
	return nil
}

func (b Baggageclaim) DestroyVolume(_ lager.Logger, handle string) error {
	b.Volumes = b.FilteredVolumes(func(v *Volume) bool {
		return v.handle != handle
	})
	return nil
}

func matchesFilter(properties map[string]string, filter map[string]string) bool {
	for k, v := range filter {
		if properties[k] != v {
			return false
		}
	}
	return true
}

func NewVolume(handle string) *Volume {
	return &Volume{
		handle: handle,
		path:   fmt.Sprintf("/volumes/%s", handle),
		Spec: baggageclaim.VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Properties: baggageclaim.VolumeProperties{},
		},
		Content: fstest.MapFS{},
	}
}

type Volume struct {
	handle string
	path   string
	Spec   baggageclaim.VolumeSpec

	Content fstest.MapFS
}

func (v Volume) WithContent(fs fstest.MapFS) *Volume {
	v.Content = fs
	return &v
}

func (v Volume) WithSpec(spec baggageclaim.VolumeSpec) *Volume {
	v.Spec = spec
	return &v
}

func (v Volume) Handle() string { return v.handle }
func (v Volume) Path() string   { return v.path }

func (v Volume) SetProperty(key, value string) error {
	v.Spec.Properties[key] = value
	return nil
}
func (v Volume) Properties() (baggageclaim.VolumeProperties, error) {
	return v.Spec.Properties, nil
}

func (v *Volume) SetPrivileged(p bool) error {
	v.Spec.Privileged = p
	return nil
}
func (v Volume) GetPrivileged() (bool, error) { return v.Spec.Privileged, nil }

func (v Volume) StreamIn(ctx context.Context, path string, encoding baggageclaim.Encoding, tarStream io.Reader) error {
	if encoding != baggageclaim.GzipEncoding {
		return errors.New("only gzip is supported for gardenruntimetest.Volume")
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
			filePath := filepath.Join(path, header.Name)
			fileData := new(bytes.Buffer)
			if _, err := io.Copy(fileData, tarReader); err != nil {
				return err
			}
			v.Content[filePath] = &fstest.MapFile{Data: fileData.Bytes()}
		default:
			panic(fmt.Sprintf("unexpected tar type: %v", header.Typeflag))
		}
	}
}
func (v Volume) StreamOut(ctx context.Context, path string, encoding baggageclaim.Encoding) (io.ReadCloser, error) {
	if encoding != baggageclaim.GzipEncoding {
		return nil, errors.New("only gzip is supported for gardenruntimetest.Volume")
	}

	buf := new(bytes.Buffer)

	gzipWriter := gzip.NewWriter(buf)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err := fs.WalkDir(v.Content, path, func(filePath string, dirEntry fs.DirEntry, err error) error {
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
		file, err := v.Content.Open(filePath)
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
	return noopCloser{buf}, nil
}

func (v Volume) GetStreamInP2pUrl(_ context.Context, _ string) (string, error) {
	panic("unimplemented")
}

func (v Volume) StreamP2pOut(_ context.Context, _ string, _ string, _ baggageclaim.Encoding) error {
	panic("unimplemented")
}

func (v Volume) Destroy() error {
	return nil
}

type noopCloser struct{ io.Reader }

func (noopCloser) Close() error { return nil }
