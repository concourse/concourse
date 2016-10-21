package resource

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Get(worker.Volume, IOConfig, atc.Source, atc.Params, atc.Version, <-chan os.Signal, chan<- struct{}) (VersionedSource, error)
	Put(IOConfig, atc.Source, atc.Params, ArtifactSource, <-chan os.Signal, chan<- struct{}) (VersionedSource, error)
	Check(atc.Source, atc.Version) ([]atc.Version, error)

	Release(*time.Duration)
}

type ResourceType string

type Session struct {
	ID        worker.Identifier
	Metadata  worker.Metadata
	Ephemeral bool
}

//go:generate counterfeiter . Cache

type Cache interface {
	IsInitialized() (bool, error)
	Initialize() error
	Volume() worker.Volume
}

type Metadata interface {
	Env() []string
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}

//go:generate counterfeiter . ArtifactSource

type ArtifactSource interface {
	StreamTo(ArtifactDestination) error

	// VolumeOn returns a Volume object that contains the artifact from the
	// ArtifactSource which is on a particular Worker. If a volume cannot be found
	// or a volume manager cannot be found on the worker then it will return
	// false.
	VolumeOn(worker.Worker) (worker.Volume, bool, error)
}

//go:generate counterfeiter . ArtifactDestination

type ArtifactDestination interface {
	StreamIn(string, io.Reader) error
}

// TODO: check if we need it
func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

type resource struct {
	container worker.Container

	ScriptFailure bool
}

func NewResourceForContainer(container worker.Container) Resource {
	return &resource{
		container: container,
	}
}

func (resource *resource) Release(finalTTL *time.Duration) {
	resource.container.Release(finalTTL)
}
