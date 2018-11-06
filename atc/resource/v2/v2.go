package v2

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
)

const TaskProcessID = "resource"

type resource struct {
	container worker.Container
	info      ResourceInfo
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

type Event struct {
	Action   string       `json:"action"`
	Version  atc.Version  `json:"version"`
	Space    atc.Space    `json:"space"`
	Metadata atc.Metadata `json:"metadata"`
}

type ActionNotFoundError struct {
	Action string
}

func (e ActionNotFoundError) Error() string {
	return fmt.Sprintf("unrecognized action: %s", e.Action)
}

func NewResource(container worker.Container, info ResourceInfo) *resource {
	return &resource{
		container: container,
		info:      info,
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
