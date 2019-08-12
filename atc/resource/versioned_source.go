package resource

import (
	"io"
	"path"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . VersionedSource

type VersionedSource interface {
	Version() atc.Version
	Metadata() []atc.MetadataField

	StreamOut(string) (io.ReadCloser, error)
	StreamIn(string, io.Reader) error

	Volume() worker.Volume
}

type VersionResult struct {
	Version atc.Version `json:"version"`

	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

func NewGetVersionedSource(volume worker.Volume, version atc.Version, metadata []atc.MetadataField) VersionedSource {
	return &getVersionedSource{
		volume:      volume,
		resourceDir: ResourcesDir("get"),

		versionResult: VersionResult{
			Version:  version,
			Metadata: metadata,
		},
	}
}

type getVersionedSource struct {
	versionResult VersionResult

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
