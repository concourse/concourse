package worker

import (
	"fmt"
	"strings"

	"github.com/concourse/atc"
)

type ContainerSpec interface {
	WorkerSpec() WorkerSpec
}

type WorkerSpec struct {
	Platform     string
	ResourceType string
	Tags         []string
}

func (spec WorkerSpec) Description() string {
	var attrs []string

	if spec.ResourceType != "" {
		attrs = append(attrs, fmt.Sprintf("resource type '%s'", spec.ResourceType))
	}

	if spec.Platform != "" {
		attrs = append(attrs, fmt.Sprintf("platform '%s'", spec.Platform))
	}

	for _, tag := range spec.Tags {
		attrs = append(attrs, fmt.Sprintf("tag '%s'", tag))
	}

	return strings.Join(attrs, ", ")
}

type ResourceTypeContainerSpec struct {
	Type                 string
	ImageResourcePointer *atc.TaskImageConfig
	Ephemeral            bool
	Tags                 []string
	Env                  []string

	// Not Copy-on-Write. Used for a single mount in Get containers.
	Cache VolumeMount

	// Copy-on-Write. Used for mounting multiple resources into a Put container.
	Mounts []VolumeMount
}

func (spec ResourceTypeContainerSpec) WorkerSpec() WorkerSpec {
	return WorkerSpec{
		ResourceType: spec.Type,
		Tags:         spec.Tags,
	}
}

type TaskContainerSpec struct {
	Platform             string
	Image                string
	ImageResourcePointer *atc.TaskImageConfig
	Privileged           bool
	Tags                 []string
	Inputs               []VolumeMount
	Outputs              []VolumeMount
}

func (spec TaskContainerSpec) WorkerSpec() WorkerSpec {
	return WorkerSpec{
		Platform: spec.Platform,
		Tags:     spec.Tags,
	}
}
