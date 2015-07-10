package resource

import (
	"io"
	"path"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

type versionResult struct {
	Version atc.Version `json:"version"`

	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type versionedSource struct {
	ifrit.Runner

	versionResult versionResult

	container garden.Container

	resourceDir string
}

func (vs *versionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *versionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *versionedSource) StreamOut(src string) (io.ReadCloser, error) {
	return vs.container.StreamOut(garden.StreamOutSpec{
		// don't use path.Join; it strips trailing slashes
		Path: vs.resourceDir + "/" + src,
	})
}

func (vs *versionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.container.StreamIn(garden.StreamInSpec{
		Path:      path.Join(vs.resourceDir, dst),
		TarStream: src,
	})
}
