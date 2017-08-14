package worker

import (
	"fmt"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
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
	Env       []string

	// Working directory for processes run in the container.
	Dir string

	// Inputs to provide to the container. Inputs with a volume local to the
	// selected worker will be made available via a COW volume; others will be
	// streamed.
	Inputs []InputSource

	// Outputs for which volumes should be created and mounted into the container.
	Outputs OutputPaths

	// Optional user to run processes as. Overwrites the one specified in the docker image.
	User string
}

// OutputPaths is a mapping from output name to its path in the container.
type OutputPaths map[string]string

type ImageSpec struct {
	ResourceType        string
	ImageURL            string
	ImageResource       *ImageResource
	ImageArtifactSource ArtifactSource
	ImageArtifactName   ArtifactName
	Privileged          bool
}

type ImageResource struct {
	Type    string
	Source  creds.Source
	Params  *atc.Params
	Version *atc.Version
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
