package worker

import (
	"fmt"
	"strings"

	"github.com/concourse/concourse/atc/runtime"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type WorkerSpec struct {
	Platform      string
	ResourceType  string
	Tags          []string
	TeamID        int
	ResourceTypes atc.VersionedResourceTypes
}

type ContainerSpec struct {
	Platform  string
	Tags      []string
	TeamID    int
	ImageSpec ImageSpec
	Env       []string
	Type      db.ContainerType

	// Working directory for processes run in the container.
	Dir string

	InputFooBars []FooBarInput

	// Inputs to provide to the container. Inputs with a volume local to the
	// selected worker will be made available via a COW volume; others will be
	// streamed.
	Inputs []InputSource

	// Outputs for which volumes should be created and mounted into the container.
	Outputs OutputPaths

	// Resource limits to be set on the container when creating in garden.
	Limits ContainerLimits

	// Local volumes to bind mount directly to the container when creating in garden.
	BindMounts []BindMountSource

	// Optional user to run processes as. Overwrites the one specified in the docker image.
	User string
}

//go:generate counterfeiter . FooBarInput

type FooBarInput interface {
	Artifact() runtime.Artifact
	DestinationPath() string
}

//go:generate counterfeiter . InputSource

type InputSource interface {
	Source() ArtifactSource
	DestinationPath() string
}

//go:generate counterfeiter . BindMountSource

type BindMountSource interface {
	VolumeOn(Worker) (garden.BindMount, bool, error)
}

// OutputPaths is a mapping from output name to its path in the container.
type OutputPaths map[string]string

type ImageSpec struct {
	ResourceType        string
	ImageURL            string
	ImageResource       *ImageResource
	ImageArtifactSource StreamableArtifactSource
	ImageArtifact       runtime.Artifact
	Privileged          bool
}

type ImageResource struct {
	Type    string
	Source  atc.Source
	Params  atc.Params
	Version atc.Version
}

type ContainerLimits struct {
	CPU    *uint64
	Memory *uint64
}

type inputSource struct {
	source ArtifactSource
	path   string
}

func (src inputSource) Source() ArtifactSource {
	return src.source
}

func (src inputSource) DestinationPath() string {
	return src.path
}

var GardenLimitDefault = uint64(0)

func (cl ContainerLimits) ToGardenLimits() garden.Limits {
	gardenLimits := garden.Limits{}
	if cl.CPU == nil {
		gardenLimits.CPU = garden.CPULimits{LimitInShares: GardenLimitDefault}
	} else {
		gardenLimits.CPU = garden.CPULimits{LimitInShares: *cl.CPU}
	}
	if cl.Memory == nil {
		gardenLimits.Memory = garden.MemoryLimits{LimitInBytes: GardenLimitDefault}
	} else {
		gardenLimits.Memory = garden.MemoryLimits{LimitInBytes: *cl.Memory}
	}
	return gardenLimits
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
