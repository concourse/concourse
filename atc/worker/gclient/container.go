package gclient

import (
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/worker/gclient/connection"
)

//go:generate counterfeiter . Container
type Container interface {
	Handle() string

	// Stop stops a container.
	//
	// If kill is false, garden stops a container by sending the processes running inside it the SIGTERM signal.
	// It then waits for the processes to terminate before returning a response.
	// If one or more processes do not terminate within 10 seconds,
	// garden sends these processes the SIGKILL signal, killing them ungracefully.
	//
	// If kill is true, garden stops a container by sending the processing running inside it a SIGKILL signal.
	//
	// It is possible to copy files in to and out of a stopped container.
	// It is only when a container is destroyed that its filesystem is cleaned up.
	//
	// Errors:
	// * None.
	Stop(kill bool) error

	// Returns information about a container.
	Info() (garden.ContainerInfo, error)

	// StreamIn streams data into a file in a container.
	//
	// Errors:
	// *  TODO.
	StreamIn(spec garden.StreamInSpec) error

	// StreamOut streams a file out of a container.
	//
	// Errors:
	// * TODO.
	StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error)

	// Returns the current bandwidth limits set for the container.
	CurrentBandwidthLimits() (garden.BandwidthLimits, error)

	// Returns the current CPU limts set for the container.
	CurrentCPULimits() (garden.CPULimits, error)

	// Returns the current disk limts set for the container.
	CurrentDiskLimits() (garden.DiskLimits, error)

	// Returns the current memory limts set for the container.
	CurrentMemoryLimits() (garden.MemoryLimits, error)

	// Map a port on the host to a port in the container so that traffic to the
	// host port is forwarded to the container port. This is deprecated in
	// favour of passing NetIn configuration in the ContainerSpec at creation
	// time.
	//
	// If a host port is not given, a port will be acquired from the server's port
	// pool.
	//
	// If a container port is not given, the port will be the same as the
	// host port.
	//
	// The resulting host and container ports are returned in that order.
	//
	// Errors:
	// * When no port can be acquired from the server's port pool.
	NetIn(hostPort, containerPort uint32) (uint32, uint32, error)

	// Whitelist outbound network traffic. This is deprecated in favour of passing
	// NetOut configuration in the ContainerSpec at creation time.
	//
	// If the configuration directive deny_networks is not used,
	// all networks are already whitelisted and this command is effectively a no-op.
	//
	// Later NetOut calls take precedence over earlier calls, which is
	// significant only in relation to logging.
	//
	// Errors:
	// * An error is returned if the NetOut call fails.
	NetOut(netOutRule garden.NetOutRule) error

	// A Bulk call for NetOut. This is deprecated in favour of passing
	// NetOut configuration in the ContainerSpec at creation time.
	//
	// Errors:
	// * An error is returned if any of the NetOut calls fail.
	BulkNetOut(netOutRules []garden.NetOutRule) error

	// Run a script inside a container.
	//
	// The root user will be mapped to a non-root UID in the host unless the container (not this process) was created with 'privileged' true.
	//
	// Errors:
	// * TODO.
	Run(context.Context, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error)

	// Attach starts streaming the output back to the client from a specified process.
	//
	// Errors:
	// * processID does not refer to a running process.
	Attach(ctx context.Context, processID string, io garden.ProcessIO) (garden.Process, error)

	// Metrics returns the current set of metrics for a container
	Metrics() (garden.Metrics, error)

	// Sets the grace time.
	SetGraceTime(graceTime time.Duration) error

	// Properties returns the current set of properties
	Properties() (garden.Properties, error)

	// Property returns the value of the property with the specified name.
	//
	// Errors:
	// * When the property does not exist on the container.
	Property(name string) (string, error)

	// Set a named property on a container to a specified value.
	//
	// Errors:
	// * None.
	SetProperty(name string, value string) error

	// Remove a property with the specified name from a container.
	//
	// Errors:
	// * None.
	RemoveProperty(name string) error
}

type container struct {
	handle string

	connection connection.Connection
}

func newContainer(handle string, connection connection.Connection) Container {
	return &container{
		handle:     handle,
		connection: connection,
	}
}

func (container *container) Handle() string {
	return container.handle
}

func (container *container) Stop(kill bool) error {
	return container.connection.Stop(container.handle, kill)
}

func (container *container) Info() (garden.ContainerInfo, error) {
	return container.connection.Info(container.handle)
}

func (container *container) StreamIn(spec garden.StreamInSpec) error {
	return container.connection.StreamIn(container.handle, spec)
}

func (container *container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	return container.connection.StreamOut(container.handle, spec)
}

func (container *container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return container.connection.CurrentBandwidthLimits(container.handle)
}

func (container *container) CurrentCPULimits() (garden.CPULimits, error) {
	return container.connection.CurrentCPULimits(container.handle)
}

func (container *container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return container.connection.CurrentDiskLimits(container.handle)
}

func (container *container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return container.connection.CurrentMemoryLimits(container.handle)
}

func (container *container) Run(ctx context.Context, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	return container.connection.Run(ctx, container.handle, spec, io)
}

func (container *container) Attach(ctx context.Context, processID string, io garden.ProcessIO) (garden.Process, error) {
	return container.connection.Attach(ctx, container.handle, processID, io)
}

func (container *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return container.connection.NetIn(container.handle, hostPort, containerPort)
}

func (container *container) NetOut(netOutRule garden.NetOutRule) error {
	return container.connection.NetOut(container.handle, netOutRule)
}

func (container *container) BulkNetOut(netOutRules []garden.NetOutRule) error {
	return container.connection.BulkNetOut(container.handle, netOutRules)
}

func (container *container) Metrics() (garden.Metrics, error) {
	return container.connection.Metrics(container.handle)
}

func (container *container) SetGraceTime(graceTime time.Duration) error {
	return container.connection.SetGraceTime(container.handle, graceTime)
}

func (container *container) Properties() (garden.Properties, error) {
	return container.connection.Properties(container.handle)
}

func (container *container) Property(name string) (string, error) {
	return container.connection.Property(container.handle, name)
}

func (container *container) SetProperty(name string, value string) error {
	return container.connection.SetProperty(container.handle, name, value)
}

func (container *container) RemoveProperty(name string) error {
	return container.connection.RemoveProperty(container.handle, name)
}
