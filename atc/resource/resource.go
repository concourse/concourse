package resource

import (
	"context"
	"io"
	"path/filepath"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, worker.Volume, IOConfig, atc.Source, atc.Params, atc.Version) (VersionedSource, error)
	Put(context.Context, IOConfig, atc.Source, atc.Params) (VersionResult, error)
	Check(context.Context, atc.Source, atc.Version) ([]atc.Version, error)
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

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewResourceForContainer(worker.Container) Resource
}

func NewResourceFactory() ResourceFactory {
	return &resourceFactory{}
}

// TODO: This factory is purely used for testing and faking out the Resource
// object. Please remove asap if possible.
type resourceFactory struct{}

func (rf *resourceFactory) NewResourceForContainer(container worker.Container) Resource {
	return &resource{
		container: container,
	}
}
