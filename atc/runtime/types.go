package runtime

import (
	"context"
	"io"
	"strings"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"go.opentelemetry.io/otel/propagation"
)

// Worker represents an entity that aggregates Containers and Volumes, and
// provides a way to construct new/lookup existing Containers and Volumes.
// Depending on the runtime implementation, a Worker may be a single VM, or may
// be a Kubernetes/Nomad cluster, etc.
type Worker interface {
	// Name gives the name of the Worker. The Name is used to uniquely identify
	// a Worker, so no two Workers may share a Name.
	Name() string

	// FindOrCreateContainer looks for a Container specified by the provided
	// db.ContainerOwner, and if none exists, creates one. This also includes
	// finding/creating the associated Volumes and properly mounting them to
	// the Container.
	//
	// It can be thought of as declaratively saying "I want a container
	// matching these specifications" - and the Worker implementation will
	// "make it so", regardless of what Containers/Volumes already exist.
	FindOrCreateContainer(context.Context, db.ContainerOwner, db.ContainerMetadata, ContainerSpec, BuildStepDelegate) (Container, []VolumeMount, error)
	// CreateVolumeForArtifact creates a new empty Volume to be used as a
	// WorkerArtifact. This is used for uploading local inputs to a worker via
	// `fly execute -i ...`.
	CreateVolumeForArtifact(ctx context.Context, teamID int) (Volume, db.WorkerArtifact, error)

	// LookupContainer finds the Container on the Worker by its handle, if it
	// exists.
	LookupContainer(ctx context.Context, handle string) (Container, bool, error)
	// LookupVolume finds the Volume on the Worker by its handle, if it exists.
	LookupVolume(ctx context.Context, handle string) (Volume, bool, error)

	DBWorker() db.Worker
}

// Container represents an entity that can execute Processes. It need not
// represent a "true" container - e.g. a Container may represent a Garden
// Container, or it may represent a Kubernetes Pod, or a Nomad Job (or Task?),
// etc.
type Container interface {
	// Run starts a Process on the Container. If the executable (defined in
	// ProcessSpec) does not exist, an ExecutableNotFound error must be
	// returned.
	//
	// The passed in Context only applies to the request to start the Process,
	// so cancelling the Context should not cancel the Process. To cancel the
	// Process, use the Context passed into Process.Wait.
	Run(context.Context, ProcessSpec, ProcessIO) (Process, error)
	// Attach attempts to attach to an existing process by its ID. An error
	// indicates either the ID doesn't correspond to a running process, or that
	// something unexpected went wrong.
	//
	// The passed in Context only applies to the request to attach to the
	// Process, so cancelling the Context should not cancel the Process. To
	// cancel the Process, use the Context passed into Process.Wait.
	Attach(context.Context, string, ProcessIO) (Process, error)

	// Properties gives the key/value pairs that were ascribed to the
	// Container, either via SetProperty() or at creation time.
	Properties() (map[string]string, error)
	// SetProperty adds a new key/value pair to the Container's Properties.
	SetProperty(name string, value string) error

	DBContainer() db.CreatedContainer
}

// ContainerSpec defines how to construct a container.
type ContainerSpec struct {
	// TeamID identifies the team to which the Container belongs.
	TeamID int
	// TeamName is the name of the team to which the Container belongs.
	TeamName string
	// JobID identifies the job in which the Container is running, used for
	// identifying task caches.
	JobID int
	// StepName is the name of the task step, used for identifying task caches.
	// If the Container is not for a task step, this may be left empty.
	StepName string

	// ImageSpec defines where the container image should come from.
	ImageSpec ImageSpec
	// Env is a list of environment variables (of the form "NAME=VALUE") to use
	// for running Processes. These are in addition to, but take precedence
	// over, those defined in the container image's metdata file.
	Env []string
	// Type is the type of step the Container is for (e.g. task, get, etc.).
	Type db.ContainerType

	// Dir is the working directory for processes run in the container. Must be
	// an absolute path.
	Dir string

	// Inputs defines the inputs to provide to the container. The Artifact
	// pointed to in these inputs may be on another worker, in which case they
	// must be streamed.
	Inputs []Input

	// Caches is a list of container paths to cache after execution and
	// re-mount to the volume. If the cached volume doesn't yet exist, an empty
	// volume should be mounted.
	//
	// Paths may be relative (to Dir) or absolute.
	Caches []string

	// Outputs defines a mapping of output names to paths within the container
	// for which volumes should be created and mounted.
	Outputs OutputPaths

	// Limits specifies resource limits to be set on the container.
	Limits ContainerLimits

	// CertsBindMount indicates whether or not to mount the worker's Certs
	// volume onto the container.
	CertsBindMount bool

	// Hermetic indicates whether or not the container has external network access.
	Hermetic bool
}

type BuildStepDelegate interface {
	StreamingVolume(lager.Logger, string, string, string)
	WaitingForStreamedVolume(lager.Logger, string, string)
	BuildStartTime() time.Time
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

// ProcessSpec defines how a process should be run in the Container.
type ProcessSpec struct {
	// ID is optional - if set, the process can be Attach'd to using this ID.
	// If unset, an ID will be generated, and can be retrieved using
	// Process.ID().
	ID string

	// Path is the executable to run (i.e. $0)
	Path string
	// Args is the list of command-line arguments to provide to the executable
	// (i.e. $1-$x)
	Args []string
	// Env is a list of environment variables (of the form "NAME=VALUE") to
	// provide to the running process. These are in addition to, but take
	// precedence over, those set in ContainerSpec.Env.
	Env []string
	// Dir is the working directory in which to run the process. This takes
	// precedence over ContainerSpec.Dir.
	Dir string
	// User is the username to run the process as. If unset, defaults to the
	// user specified by the container image's metadata.
	User string
	// TTY defines the TTY allocated to the process.
	TTY *TTYSpec
}

// TTYSpec defines the TTY allocated to a Process. It may be set when starting
// the Process via the ProcessSpec, or at runtime using Process.SetTTY().
type TTYSpec struct {
	WindowSize WindowSize
}

// WindowSize represents the size of the TTY allocated to the Process, measured
// in characters.
type WindowSize struct {
	Columns uint16
	Rows    uint16
}

type ProcessIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Process represents a running process in a Container. Containers may have
// multiple Processes running simultaneously, but typically just have one (task
// script, resource script, etc.). Hijacking is run as a separate Process.
type Process interface {
	// ID gives an identifier for the process that can be used in Container.Attach().
	ID() string
	// Wait waits for the process to exit and returns the result. A failed
	// script does not return an error - only a non-zero ExitStatus in the
	// ProcessResult.
	Wait(context.Context) (ProcessResult, error)
	// SetTTY updates the TTY of the Process dynamically, e.g. to resize the
	// terminal.
	SetTTY(tty TTYSpec) error
}

// ProcessResult is the result of executing a Process.
type ProcessResult struct {
	ExitStatus int
}

// Input represents a Volume (typically from a build artifact) to mount to the
// container.
type Input struct {
	// Artifact is the artifact to mount. This artifact may need to be
	// streamed (if it is not a Volume on the target worker).
	Artifact Artifact
	// DestinationPath is the path in the container to mount the input.
	//
	// May be absolute or relative to ContainerSpec.Dir.
	DestinationPath string
	// FromCache indicates if the artifact is found from cache or not.
	// If an artifact is found from cache, then if it should not be considered
	// by volume-locality strategy.
	FromCache bool
}

// OutputPaths is a mapping from output name to its path in the container.
type OutputPaths map[string]string

// ImageSpec defines where the container image should come from. For
// containerized platforms, only one of ImageVolume, ImageURL, or ResourceType
// must be set. For non-containerized platforms, each of those fields should be
// left empty.
type ImageSpec struct {
	// ImageArtifact is the Artifact whose contents are the container image.
	ImageArtifact Artifact
	// ImageURL points to a path on the worker to use as the rootfs. This
	// corresponds with `task.rootfs_uri`:
	// https://concourse-ci.org/tasks.html#schema.task.rootfs_uri
	ImageURL string
	// ResourceType is the name of the base resource type to use for the image.
	ResourceType string

	// Privileged indicates whether the container should be privileged. The
	// precise meaning of "privileged" is runtime specific.
	Privileged bool
}

// ContainerLimits defines resource limits for a Container.
type ContainerLimits struct {
	// CPU defines the CPU limit for all Processes run in the Container,
	// measured in shares. Unset means no limit.
	CPU *uint64
	// Memory defines the memory limit for all Processes run in the Container,
	// measured in bytes. Unset means no limit.
	Memory *uint64
}

// Artifact represents an output from a step that can be used as an input to
// other steps.
type Artifact interface {
	// StreamOut converts the contents of the Artifact under path to a compressed
	// tar stream. The result of StreamOut can be passed in to StreamIn for
	// another Artifact.
	//
	// path is a relative path - "." indicates using the root of the Artifact.
	StreamOut(ctx context.Context, path string, compression compression.Compression) (io.ReadCloser, error)

	// Handle gives the globally unique ID of the Volume.
	Handle() string

	// Source gives the original source of the artifact e.g. worker name
	Source() string
}

// Volume represents a data volume that can be mounted to a Container.
type Volume interface {
	Artifact

	// StreamIn accepts a compressed tar stream and populates the contents of
	// the Volume with the contents under path. This tar stream can be the
	// result of a StreamOut call for another Volume.
	//
	// path is a relative path - "." indicates using the root of the Volume.
	StreamIn(ctx context.Context, path string, compression compression.Compression, limitInMB float64, reader io.Reader) error

	// InitializeResourceCache is called upon a successful run of the get step
	// to register this Volume as a resource cache.
	InitializeResourceCache(ctx context.Context, urc db.ResourceCache) (*db.UsedWorkerResourceCache, error)

	// InitializeStreamedResourceCache is called when an external resource
	// cache volume is streamed locally to register this volume as a resource
	// cache.
	InitializeStreamedResourceCache(ctx context.Context, urc db.ResourceCache, sourceWorkerResourceCacheID int) (*db.UsedWorkerResourceCache, error)

	// InitializeTaskCache is called upon a successful run of the task step to
	// register this Volume as a task cache.
	InitializeTaskCache(ctx context.Context, jobID int, stepName string, path string, privileged bool) error

	DBVolume() db.CreatedVolume
}

// P2PVolume is an interface that may also be satisfied by Volume
// implementations. When streaming contents from one Volume to another, if both
// Volumes implement this interface, then the P2P streaming methods will be
// used.
type P2PVolume interface {
	Volume

	// GetStreamInP2PURL gives a URL which, if you POST to with a compressed
	// tar stream (using the encoding format specified by the
	// `Content-Encoding` header parameter - either "gzip" or "zstd"), will
	// stream those contents into the Volume under path.
	//
	// path is a relative path - "." indicates using the root of the Volume.
	GetStreamInP2PURL(ctx context.Context, path string) (string, error)

	// StreamP2POut takes in a URL generated by another P2PVolume's
	// GetStreamInP2PURL call, and makes a POST request with the compressed tar
	// stream of the contents of the Volume under path. The encoding format is
	// specified by the `Content-Encoding` header (either "gzip" or "zstd").
	//
	// path is a relative path - "." indicates using the root of the Volume.
	StreamP2POut(ctx context.Context, path string, destURL string, compression compression.Compression) error
}

// VolumeMount defines a Volume mounted at a particular path in a Container.
type VolumeMount struct {
	// Volume is the mounted Volume.
	Volume Volume
	// MountPath is the absolute path in the Container at which the Volume is
	// mounted.
	MountPath string
}
