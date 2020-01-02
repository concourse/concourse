package backend

import (
	"io"
	"time"

	"code.cloudfoundry.org/garden"
)

type Container struct{
	handle string
}

// func NewContainer() Container {
// 	return Container{}
// }

var _ garden.Container = (*Container)(nil)

func (c *Container) Handle() (handle string) { return c.handle }

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
func (c *Container) Stop(kill bool) (err error) { return }

// Returns information about a container.
func (c *Container) Info() (info garden.ContainerInfo, err error) { return }

// StreamIn streams data into a file in a container.
//
// Errors:
// *  TODO.
func (c *Container) StreamIn(spec garden.StreamInSpec) (err error) { return }

// StreamOut streams a file out of a container.
//
// Errors:
// * TODO.
func (c *Container) StreamOut(spec garden.StreamOutSpec) (readCloser io.ReadCloser, err error) { return }

// Returns the current bandwidth limits set for the container.
func (c *Container) CurrentBandwidthLimits() (limits garden.BandwidthLimits, err error) { return }

// Returns the current CPU limts set for the container.
func (c *Container) CurrentCPULimits() (limits garden.CPULimits, err error) { return }

// Returns the current disk limts set for the container.
func (c *Container) CurrentDiskLimits() (limits garden.DiskLimits, err error) { return }

// Returns the current memory limts set for the container.
func (c *Container) CurrentMemoryLimits() (limits garden.MemoryLimits, err error) { return }

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
func (c *Container) NetIn(hostPort, containerPort uint32) (a, b uint32, err error) { return }

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
func (c *Container) NetOut(netOutRule garden.NetOutRule) (err error) { return }

// A Bulk call for NetOut. This is deprecated in favour of passing
// NetOut configuration in the ContainerSpec at creation time.
//
// Errors:
// * An error is returned if any of the NetOut calls fail.
func (c *Container) BulkNetOut(netOutRules []garden.NetOutRule) (err error) { return }

// Run a script inside a container.
//
// The root user will be mapped to a non-root UID in the host unless the
// container (not this process) was created with 'privileged' true.
//
// Errors:
// * TODO.
func (c *Container) Run(garden.ProcessSpec, garden.ProcessIO) (process garden.Process, err error) {
	return
}

// Attach starts streaming the output back to the client from a specified process.
//
// Errors:
// * processID does not refer to a running process.
func (c *Container) Attach(processID string, io garden.ProcessIO) (process garden.Process, err error) {
	return
}

// Metrics returns the current set of metrics for a container
func (c *Container) Metrics() (metrics garden.Metrics, err error) { return }

// Sets the grace time.
func (c *Container) SetGraceTime(graceTime time.Duration) (err error) { return }

// Properties returns the current set of properties
func (c *Container) Properties() (properties garden.Properties, err error) { return }

// Property returns the value of the property with the specified name.
//
// Errors:
// * When the property does not exist on the container.
func (c *Container) Property(name string) (value string, err error) { return }

// Set a named property on a container to a specified value.
//
// Errors:
// * None.
func (c *Container) SetProperty(name string, value string) (err error) { return }

// Remove a property with the specified name from a container.
//
// Errors:
// * None.
func (c *Container) RemoveProperty(name string) (err error) { return }
