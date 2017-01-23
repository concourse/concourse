package resource

import (
	"io"
	"os"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Get(worker.Volume, IOConfig, atc.Source, atc.Params, atc.Version, <-chan os.Signal, chan<- struct{}) (VersionedSource, error)
	Put(IOConfig, atc.Source, atc.Params, worker.ArtifactSource, <-chan os.Signal, chan<- struct{}) (VersionedSource, error)
	Check(atc.Source, atc.Version) ([]atc.Version, error)
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
