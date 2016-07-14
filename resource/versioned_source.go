package resource

import (
	"io"
	"path"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/tedsuo/ifrit"
)

type versionResult struct {
	Version atc.Version `json:"version"`

	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type getVersionedSource struct {
	ifrit.Runner

	versionResult versionResult

	volume      worker.Volume
	resourceDir string
}

type putVersionedSource struct {
	ifrit.Runner

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

func (vs *putVersionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.container.StreamIn(garden.StreamInSpec{
		Path:      path.Join(vs.resourceDir, dst),
		TarStream: src,
	})
}

func (vs *getVersionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *getVersionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *getVersionedSource) StreamOut(src string) (io.ReadCloser, error) {
	return vs.volume.StreamOut(src)
}

func (vs *getVersionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.volume.StreamIn(
		path.Join(vs.resourceDir, dst),
		src,
	)
}
