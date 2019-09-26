package resource

import (
	"context"
	"path/filepath"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, runtime.ProcessSpec, runtime.Runnable) (runtime.VersionResult, error)
	Put(context.Context, runtime.ProcessSpec, runtime.Runnable) (runtime.VersionResult, error)
	Check(context.Context, runtime.ProcessSpec, runtime.Runnable) ([]atc.Version, error)

	Source() atc.Source
	Params() atc.Params
	Version() atc.Version
}

type ResourceType string

type Metadata interface {
	Env() []string
}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

//func NewResource(spec runtime.ProcessSpec, configParams ConfigParams) *resource {
//	return &resource{
//		processSpec: spec,
//		params:      configParams,
//	}
//}

func NewResource(source atc.Source, params atc.Params, version atc.Version) Resource {
	return &resource{
		source:  source,
		params:  params,
		version: version,
	}
}

type resource struct {
	source  atc.Source  `json:"source"`
	params  atc.Params  `json:"params,omitempty"`
	version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Source() atc.Source {
	return resource.source
}

func (resource *resource) Params() atc.Params {
	return resource.params
}

func (resource *resource) Version() atc.Version {
	return resource.version
}
