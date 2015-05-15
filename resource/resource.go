package resource

import (
	"io"
	"path/filepath"
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Type() ResourceType

	Get(IOConfig, atc.Source, atc.Params, atc.Version) VersionedSource
	Put(IOConfig, atc.Source, atc.Params, ArtifactSource) VersionedSource

	Check(atc.Source, atc.Version) ([]atc.Version, error)

	Release()
	Destroy() error
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

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

type resource struct {
	container worker.Container
	typ       ResourceType

	releaseOnce sync.Once
}

func NewResource(
	container worker.Container,
	typ ResourceType,
) Resource {
	return &resource{
		container: container,
		typ:       typ,
	}
}

func (resource *resource) Type() ResourceType {
	return resource.typ
}

func (resource *resource) Release() {
	resource.container.Release()
}

func (resource *resource) Destroy() error {
	var err error

	resource.releaseOnce.Do(func() {
		err = resource.container.Destroy()
	})

	return err
}
