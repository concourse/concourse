package worker

import (
	"fmt"
	"io"
	"strings"

	"github.com/concourse/atc"
)

type WorkerSpec struct {
	Platform     string
	ResourceType string
	Tags         []string
	TeamID       int
}

type ContainerSpec struct {
	Platform  string
	Tags      []string
	TeamID    int
	ImageSpec ImageSpec
	Ephemeral bool
	Env       []string

	// Not Copy-on-Write. Used for a single mount in Get containers.
	Inputs []VolumeMount

	Outputs []VolumeMount

	// volumes that need to be mounted to container
	Mounts []VolumeMount

	// Optional user to run processes as. Overwrites the one specified in the docker image.
	User string
}

type ImageSpec struct {
	ResourceType           string
	ImageURL               string
	ImageResource          *atc.ImageResource
	ImageVolumeAndMetadata ImageVolumeAndMetadata
	Privileged             bool
}

type ImageVolumeAndMetadata struct {
	Volume         Volume
	MetadataReader io.ReadCloser
}

func (spec ContainerSpec) WorkerSpec() WorkerSpec {
	return WorkerSpec{
		ResourceType: spec.ImageSpec.ResourceType,
		Platform:     spec.Platform,
		Tags:         spec.Tags,
		TeamID:       spec.TeamID,
	}
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
