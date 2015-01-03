package resource

import (
	"io"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . Resource
type Resource interface {
	Get(IOConfig, atc.Source, atc.Params, atc.Version) VersionedSource
	Put(IOConfig, atc.Source, atc.Params, ArtifactSource) VersionedSource

	Check(atc.Source, atc.Version) ([]atc.Version, error)

	Release() error
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}

//go:generate counterfeiter . ArtifactSource
type ArtifactSource interface {
	StreamTo(ArtifactDestination) error
}

//go:generate counterfeiter . ArtifactDestination
type ArtifactDestination interface {
	StreamIn(string, io.Reader) error
}

//go:generate counterfeiter . VersionedSource
type VersionedSource interface {
	ifrit.Runner

	Version() atc.Version
	Metadata() []atc.MetadataField

	StreamOut(string) (io.ReadCloser, error)
	StreamIn(string, io.Reader) error
}

const ResourcesDir = "/tmp/build/src"

type resource struct {
	container    garden.Container
	gardenClient garden.Client
}

func NewResource(
	container garden.Container,
	gardenClient garden.Client,
) Resource {
	return &resource{
		container:    container,
		gardenClient: gardenClient,
	}
}

func (resource *resource) Release() error {
	return resource.gardenClient.Destroy(resource.container.Handle())
}
