package resource

import (
	"context"
	"path/filepath"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, runtime.Runnable) (runtime.VersionResult, error)
	Put(context.Context, runtime.Runnable) (runtime.VersionResult, error)
	Check(context.Context, runtime.Runnable) ([]atc.Version, error)
}

type ResourceType string

type Metadata interface {
	Env() []string
}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

func NewResource(spec runtime.ProcessSpec, resourceParams Params) *resource {
	return &resource{
		processSpec: spec,
		params:      resourceParams,
	}
}

type resource struct {
	processSpec runtime.ProcessSpec
	params      Params
}

type Params struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}
