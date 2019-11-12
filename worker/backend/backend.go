package backend

import (
	"time"

	"code.cloudfoundry.org/garden"
	_ "github.com/concourse/concourse/worker/backend/libcontainerd"
)

var _ garden.Backend = (*Backend)(nil)

type Backend struct{}

func New() Backend {
	return Backend{}
}

// setup?
//
func (b *Backend) Start() (err error) {
	return
}

// teardown?
//
func (b *Backend) Stop() {
	return
}

// ??
//
func (b *Backend) GraceTime(container garden.Container) (duration time.Duration) {
	return
}

// Pings the garden server in order to check connectivity.
//
// The server may, optionally, respond with specific errors indicating health
// issues.
//
// Errors:
// * garden.UnrecoverableError indicates that the garden server has entered an error state from which it cannot recover
//
// TODO - we might use the `version` service here as a proxy to "ping"
func (b *Backend) Ping() (err error) {
	return
}

// Capacity returns the physical capacity of the server's machine.
//
// Errors:
// * None.
func (b *Backend) Capacity() (capacity garden.Capacity, err error) { return }

// Create creates a new container.
//
// Errors:
// * When the handle, if specified, is already taken.
// * When one of the bind_mount paths does not exist.
// * When resource allocations fail (subnet, user ID, etc).
func (b *Backend) Create(spec garden.ContainerSpec) (container garden.Container, err error) {

	// concourse image format:
	// - {env, user} comes from a metadata format
	// - rootfs comes from a well known writable location
	// - bind mounts
	//   - e.g., to mount certs, `/scratch` ...
	//   - how do we specify these?
	// - network
	//   - how do we specify this?
	// - env
	// - properties
	//   - which ones do we add?
	// - gracetime
	//   - do we use this?
	// - what else?

	return
}

// Destroy destroys a container.
//
// When a container is destroyed, its resource allocations are released,
// its filesystem is removed, and all references to its handle are removed.
//
// All resources that have been acquired during the lifetime of the container are released.
// Examples of these resources are its subnet, its UID, and ports that were redirected to the container.
//
// TODO: list the resources that can be acquired during the lifetime of a container.
//
// Errors:
// * TODO.
func (b *Backend) Destroy(handle string) (err error) { return }

// Containers lists all containers filtered by Properties (which are ANDed together).
//
// Errors:
// * None.
func (b *Backend) Containers(properties garden.Properties) (containers []garden.Container, err error) {
	return
}

// BulkInfo returns info or error for a list of containers.
func (b *Backend) BulkInfo(handles []string) (info map[string]garden.ContainerInfoEntry, err error) {
	return
}

// BulkMetrics returns metrics or error for a list of containers.
func (b *Backend) BulkMetrics(handles []string) (metrics map[string]garden.ContainerMetricsEntry, err error) {
	return
}

// Lookup returns the container with the specified handle.
//
// Errors:
// * Container not found.
func (b *Backend) Lookup(handle string) (container garden.Container, err error) { return }
