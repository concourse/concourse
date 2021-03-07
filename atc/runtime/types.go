package runtime

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"hash/adler32"
	"io"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
)

const (
	ResourceResultPropertyName = "concourse:resource-result"
	ResourceProcessID          = "resource"
)

//go:generate counterfeiter . StartingEventDelegate
type StartingEventDelegate interface {
	Starting(lager.Logger)
}

/////////////////

type Worker interface {
	Name() string
	FindOrCreateContainer(context.Context, db.ContainerOwner, db.ContainerMetadata, ContainerSpec) (Container, []VolumeMount, error)
	LookupVolume(logger lager.Logger, handle string) (Volume, bool, error)
}

type Container interface {
	Run(context.Context, ProcessSpec, ProcessIO) (ProcessResult, error)
	Attach(context.Context, ProcessSpec, ProcessIO) (ProcessResult, error)

	Properties() (map[string]string, error)
	SetProperty(name string, value string) error
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
	StreamIn(ctx context.Context, path string, compression compression.Compression, reader io.Reader) error
	StreamOut(ctx context.Context, path string, compression compression.Compression) (io.ReadCloser, error)

	DBVolume() db.CreatedVolume
}

type P2PVolume interface {
	Handle() string
	GetStreamInP2PURL(ctx context.Context, path string) (string, error)
	StreamP2POut(ctx context.Context, path string, destURL string, compression compression.Compression) error
}

type VolumeSpec struct {
	Strategy   VolumeStrategy
	Properties map[string]string
	Privileged bool
}

type VolumeStrategy interface {
	Encode() *json.RawMessage
}

type VolumeMount struct {
	MountPath string
	Volume    Volume
}
