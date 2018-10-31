package v2

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
)

type resource struct {
	container      worker.Container
	info           ResourceInfo
	resourceConfig db.ResourceConfig
}

type ResourceInfo struct {
	Artifacts Artifacts
}

type Artifacts struct {
	APIVersion string `json:"api_version"`
	Check      string `json:"check"`
	Get        string `json:"get"`
	Put        string `json:"put"`
}

func NewResource(container worker.Container, info ResourceInfo, resourceConfig db.ResourceConfig) *resource {
	return &resource{
		container:      container,
		info:           info,
		resourceConfig: resourceConfig,
	}
}

func (r *resource) Container() worker.Container {
	return r.container
}

// XXX: Maybe make it into a v2.Config type?
func constructConfig(source atc.Source, params atc.Params) map[string]interface{} {
	config := map[string]interface{}{}

	for k, v := range source {
		config[k] = v
	}

	for k, v := range params {
		config[k] = v
	}

	return config
}
