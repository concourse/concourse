package worker

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/db"
	"go.opentelemetry.io/otel/propagation"
)

type WorkerSpec struct {
	Platform     string
	ResourceType string
	Tags         []string
	TeamID       int
}

type ContainerSpec struct {
	TeamID    int
	ImageSpec ImageSpec
	Env       []string
	Type      db.ContainerType

	// Working directory for processes run in the container.
	Dir string

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

// ContainerSpec must implement propagation.TextMapCarrier so that it can be
// used to pass tracing metadata to the containers via env vars (specifically,
// the TRACEPARENT var)
var _ propagation.TextMapCarrier = new(ContainerSpec)

func (cs *ContainerSpec) Get(key string) string {
	for _, env := range cs.Env {
		assignment := strings.SplitN("=", env, 2)
		if assignment[0] == strings.ToUpper(key) {
			return assignment[1]
		}
	}
	return ""
}

func (cs *ContainerSpec) Set(key string, value string) {
	varName := strings.ToUpper(key)
	envVar := varName + "=" + value
	for i, env := range cs.Env {
		if strings.SplitN("=", env, 2)[0] == varName {
			cs.Env[i] = envVar
			return
		}
	}
	cs.Env = append(cs.Env, envVar)
}

func (cs *ContainerSpec) Keys() []string {
	// this implementation isn't technically correct - it gives all environment
	// vars as the list of keys (rather than just those set by Set), and it
	// assumes the original keys were all lowercased). from what I can tell,
	// though, this Keys method isn't currently even used, so this doesn't
	// matter right now (but may in the future...)
	keys := make([]string, len(cs.Env))
	for i, env := range cs.Env {
		envName := strings.SplitN("=", env, 2)[0]
		keys[i] = strings.ToLower(envName)
	}
	return keys
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
	ImageArtifactSource StreamableArtifactSource
	Privileged          bool
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
