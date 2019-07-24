package resource

import (
	"context"
	"path/filepath"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewResourceForContainer(container worker.Container) Resource
}

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, worker.Volume, runtime.IOConfig, atc.Source, atc.Params, atc.Version) (VersionedSource, error)
	Put(context.Context, runtime.IOConfig, atc.Source, atc.Params) (runtime.VersionResult, error)
	Check(context.Context, atc.Source, atc.Version) ([]atc.Version, error)
}

type ResourceType string

type Metadata interface {
	Env() []string
}

//type IOConfig struct {
//	Stdout io.Writer
//	Stderr io.Writer
//}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

func NewResource(container worker.Container) *resource {
	return &resource{
		container: container,
	}
}

type resource struct {
	// TODO make this wrap a Runnable instead of a container
	container worker.Container
}

func NewResourceFactory() *resourceFactory {
	return &resourceFactory{}
}

type resourceFactory struct{}

func (rf *resourceFactory) NewResourceForContainer(container worker.Container) Resource {
	return NewResource(container)
}
