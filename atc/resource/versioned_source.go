package resource

import (
	"io"
	"path"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . VersionedSource

type VersionedSource interface {
	Version() atc.Version
	Metadata() []atc.MetadataField

	StreamOut(string) (io.ReadCloser, error)
	StreamIn(string, io.Reader) error

	Volume() worker.Volume
}

type versionResult struct {
	Version atc.Version `json:"version"`

	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type putVersionedSource struct {
	versionResult versionResult

	container garden.Container

	resourceDir string
}

func (vs *putVersionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *putVersionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *putVersionedSource) StreamOut(src string) (io.ReadCloser, error) {
	return vs.container.StreamOut(garden.StreamOutSpec{
		// don't use path.Join; it strips trailing slashes
		Path: vs.resourceDir + "/" + src,
	})
}

func (vs *putVersionedSource) Volume() worker.Volume {
	return nil
}

func (vs *putVersionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.container.StreamIn(garden.StreamInSpec{
		Path:      path.Join(vs.resourceDir, dst),
		TarStream: src,
	})
}

func NewGetVersionedSource(volume worker.Volume, version atc.Version, metadata []atc.MetadataField) VersionedSource {
	return &getVersionedSource{
		volume:      volume,
		resourceDir: ResourcesDir("get"),

		versionResult: versionResult{
			Version:  version,
			Metadata: metadata,
		},
	}
}

type getVersionedSource struct {
	versionResult versionResult

	volume      worker.Volume
	resourceDir string
}

func (vs *getVersionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *getVersionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *getVersionedSource) StreamOut(src string) (io.ReadCloser, error) {
	readCloser, err := vs.volume.StreamOut(src)
	if err != nil {
		return nil, err
	}

	return readCloser, err
}

func (vs *getVersionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.volume.StreamIn(
		path.Join(vs.resourceDir, dst),
		src,
	)
}

func (vs *getVersionedSource) Volume() worker.Volume {
	return vs.volume
}
