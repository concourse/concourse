package runtime

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"io"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	ResourceResultPropertyName = "concourse:resource-result"
	ResourceProcessID          = "resource"
)

//counterfeiter:generate . StartingEventDelegate
type StartingEventDelegate interface {
	Starting(lager.Logger)
}

type VersionResult struct {
	Version  atc.Version         `json:"version"`
	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

//go:generate counterfeiter . Artifact
type Artifact interface {
	ID() string
}

type CacheArtifact struct {
	TeamID   int
	JobID    int
	StepName string
	Path     string
}

func (art CacheArtifact) ID() string {
	return fmt.Sprintf("%d, %d, %s, %s", art.TeamID, art.JobID, art.StepName, art.Path)
}

// TODO (Krishna/Sameer): get rid of these - can GetArtifact and TaskArtifact be merged ?
type GetArtifact struct {
	VolumeHandle string
}

func (art GetArtifact) ID() string {
	return art.VolumeHandle
}

type TaskArtifact struct {
	VolumeHandle string
}

func (art TaskArtifact) ID() string {
	return art.VolumeHandle
}

// TODO (runtime/#4910): consider a different name as this is close to "Runnable" in atc/engine/engine
//go:generate counterfeiter . Runner

// TODO: get rid of this in favour of Container
type Runner interface {
	RunScript(
		ctx context.Context,
		path string,
		args []string,
		input []byte,
		output interface{},
		logDest io.Writer,
		recoverable bool,
	) error
}

/////////////////

type Worker interface {
	Name() string
	FindOrCreateContainer(context.Context, db.ContainerOwner, db.ContainerMetadata, ContainerSpec) (Container, error)
	LookupVolume(logger lager.Logger, handle string) (Volume, bool, error)
}

type Container interface {
	Run(context.Context, ProcessSpec, ProcessIO) (ProcessResult, error)
	Attach(context.Context, ProcessSpec, ProcessIO) (ProcessResult, error)

	VolumeMounts() []VolumeMount
}

type ContainerSpec struct {
	TeamID   int
	JobID    int
	StepName string

	ImageSpec ImageSpec
	Env       []string
	Type      db.ContainerType

	// Working directory for processes run in the container.
	Dir string

	// Inputs to provide to the container. Inputs with a volume local to the
	// selected worker will be made available via a COW volume; others will be
	// streamed.
	Inputs []Input

	// List cached container paths to mount to the volume if already present on
	// the worker.
	// TODO: This should replace exec.TaskStep.registerCaches.
	Caches []string

	// Outputs for which volumes should be created and mounted into the container.
	Outputs OutputPaths

	// Resource limits to be set on the container.
	Limits ContainerLimits

	// Whether or not to mount the Certs volume onto the container.
	// TODO: find a more general pattern here
	CertsBindMount bool
}

// The below methods cause ContainerSpec to fulfill the
// go.opentelemetry.io/otel/api/propagation.HTTPSupplier interface

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

type ProcessSpec struct {
	Path string
	Args []string
	Dir  string
	User string
}

func (p ProcessSpec) ID() int {
	buf := new(bytes.Buffer)

	enc := gob.NewEncoder(buf)
	enc.Encode(p)

	return int(adler32.Checksum(buf.Bytes()))
}

type ProcessIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type ProcessResult struct {
	ExitStatus int
}

type Input struct {
	VolumeHandle    string
	DestinationPath string
}

// OutputPaths is a mapping from output name to its path in the container.
type OutputPaths map[string]string

type ImageSpec struct {
	ResourceType string
	ImageURL     string
	ImageVolume  string
	Privileged   bool
}

type ContainerLimits struct {
	CPU    *uint64
	Memory *uint64
}

type Volume interface {
	Handle() string
	StreamIn(ctx context.Context, path string, encoding Encoding, reader io.Reader) error
	StreamOut(ctx context.Context, path string, encoding Encoding) (io.ReadCloser, error)
}

type VolumeSpec struct {
	Strategy   VolumeStrategy
	Properties map[string]string
	Privileged bool
}

type Encoding string

const (
	GzipEncoding Encoding = "gzip"
	ZstdEncoding Encoding = "zstd"
)

type VolumeStrategy interface {
	Encode() *json.RawMessage
}

type VolumeMount struct {
	MountPath string
	Volume    Volume
}
