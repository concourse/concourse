package resource

import (
	"context"
	"io"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, worker.Volume, IOConfig, atc.Source, atc.Params, atc.Version) (VersionedSource, error)
	Put(context.Context, IOConfig, atc.Source, atc.Params) (VersionedSource, error)
	Check(atc.Source, atc.Version) ([]atc.Version, error)
	Container() worker.Container
}

type ResourceType string

type Session struct {
	Metadata db.ContainerMetadata
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

func (r *resource) Container() worker.Container {
	return r.container
}
