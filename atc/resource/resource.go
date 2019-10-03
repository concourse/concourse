package resource

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, runtime.ProcessSpec, runtime.Runner) (runtime.VersionResult, error)
	Put(context.Context, runtime.ProcessSpec, runtime.Runner) (runtime.VersionResult, error)
	Check(context.Context, runtime.ProcessSpec, runtime.Runner) ([]atc.Version, error)
	Signature() ([]byte, error)
}

type ResourceType string

type Metadata interface {
	Env() []string
}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

func NewResource(source atc.Source, params atc.Params, version atc.Version) Resource {
	return &resource{
		Source:  source,
		Params:  params,
		Version: version,
	}
}

type resource struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Signature() ([]byte, error) {
	return json.Marshal(resource)
}
