package resource

import (
	"io"
	"path"

	garden "github.com/cloudfoundry-incubator/garden/api"
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
}

func (vs *versionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *versionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *versionedSource) StreamOut(src string) (io.ReadCloser, error) {
	return vs.container.StreamOut(path.Join(ResourcesDir, src))
}

func (vs *versionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.container.StreamIn(path.Join(ResourcesDir, dst), src)
}
