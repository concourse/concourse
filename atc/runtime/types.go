package runtime

import (
	"bytes"
	"context"
	"encoding/gob"
	"hash/adler32"
	"io"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"go.opentelemetry.io/otel/propagation"
)

type Worker interface {
	Name() string
	FindOrCreateContainer(context.Context, db.ContainerOwner, db.ContainerMetadata, ContainerSpec) (Container, []VolumeMount, error)
	CreateVolumeForArtifact(logger lager.Logger, teamID int) (Volume, db.WorkerArtifact, error)

	LookupContainer(logger lager.Logger, handle string) (Container, bool, error)
	LookupVolume(logger lager.Logger, handle string) (Volume, bool, error)
}

type Container interface {
	// Run starts a Process on the Container. If the executable (defined in
	// ProcessSpec) does not exist, an ExecutableNotFound error must be
	// returned.
	Run(context.Context, ProcessSpec, ProcessIO) (Process, error)
	// Attach attempts to attach to an existing process that was defined by the
	// ProcessSpec.
	Attach(context.Context, ProcessSpec, ProcessIO) (Process, error)

	Properties() (map[string]string, error)
	SetProperty(name string, value string) error

	DBContainer() db.CreatedContainer
}

type ContainerSpec struct {
	TeamID   int
	TeamName string
	JobID    int
	StepName string

	ImageSpec ImageSpec
	Env       []string
	Type      db.ContainerType

	// Working directory for processes run in the container. Must be an absolute path.
	Dir string

	// Inputs to provide to the container. Inputs with a volume local to the
	// selected worker will be made available via a COW volume; others will be
	// streamed.
	Inputs []Input

	// List of cached container paths to re-mount to the volume. If the cached
	// volume doesn't yet exist, an empty volume should be mounted.
	//
	// Paths may be relative (to Dir) or absolute.
	Caches []string

	// Outputs for which volumes should be created and mounted into the container.
	Outputs OutputPaths

	// Resource limits to be set on the container.
	Limits ContainerLimits

	// Whether or not to mount the Certs volume onto the container.
	// TODO: find a more general pattern here
	CertsBindMount bool
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

type ProcessSpec struct {
	Path string
	Args []string
	Env  []string
	Dir  string
	User string
	TTY  *TTYSpec
}

func (p ProcessSpec) ID() int {
	buf := new(bytes.Buffer)

	enc := gob.NewEncoder(buf)
	enc.Encode(p)

	return int(adler32.Checksum(buf.Bytes()))
}

type TTYSpec struct {
	WindowSize WindowSize
}

type WindowSize struct {
	Columns int
	Rows    int
}

type ProcessIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Process interface {
	Wait(context.Context) (ProcessResult, error)
	SetTTY(tty TTYSpec) error
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

	InitializeResourceCache(lager.Logger, db.UsedResourceCache) error
	InitializeTaskCache(logger lager.Logger, jobID int, stepName string, path string, privileged bool) error

	DBVolume() db.CreatedVolume
}

type P2PVolume interface {
	Handle() string
	GetStreamInP2PURL(ctx context.Context, path string) (string, error)
	StreamP2POut(ctx context.Context, path string, destURL string, compression compression.Compression) error
}

type VolumeMount struct {
	MountPath string
	Volume    Volume
}
