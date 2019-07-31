package gclient

import (
	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/worker/gclient/connection"
)

//go:generate counterfeiter . Client
type Client interface {
	// Pings the garden server. Checks connectivity to the server. The server may, optionally, respond with specific
	// errors indicating health issues.
	//
	// Errors:
	// * garden.UnrecoverableError indicates that the garden server has entered an error state from which it cannot recover
	Ping() error

	// Capacity returns the physical capacity of the server's machine.
	//
	// Errors:
	// * None.
	Capacity() (garden.Capacity, error)

	// Create creates a new container.
	//
	// Errors:
	// * When the handle, if specified, is already taken.
	// * When one of the bind_mount paths does not exist.
	// * When resource allocations fail (subnet, user ID, etc).
	Create(garden.ContainerSpec) (Container, error)

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
	Destroy(handle string) error

	// Containers lists all containers filtered by Properties (which are ANDed together).
	//
	// Errors:
	// * None.
	Containers(garden.Properties) ([]Container, error)

	// BulkInfo returns info or error for a list of containers.
	BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error)

	// BulkMetrics returns metrics or error for a list of containers.
	BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error)

	// Lookup returns the container with the specified handle.
	//
	// Errors:
	// * Container not found.
	Lookup(handle string) (Container, error)
}

type client struct {
	connection connection.Connection
}

func NewClient(connection connection.Connection) Client {
	return &client{
		connection: connection,
	}
}

func (client *client) Ping() error {
	return client.connection.Ping()
}

func (client *client) Capacity() (garden.Capacity, error) {
	return client.connection.Capacity()
}

func (client *client) Create(spec garden.ContainerSpec) (Container, error) {
	handle, err := client.connection.Create(spec)
	if err != nil {
		return nil, err
	}

	return newContainer(handle, client.connection), nil
}

func (client *client) Containers(properties garden.Properties) ([]Container, error) {
	handles, err := client.connection.List(properties)
	if err != nil {
		return nil, err
	}

	containers := []Container{}
	for _, handle := range handles {
		containers = append(containers, newContainer(handle, client.connection))
	}

	return containers, nil
}

func (client *client) Destroy(handle string) error {
	err := client.connection.Destroy(handle)

	return err
}

func (client *client) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return client.connection.BulkInfo(handles)
}

func (client *client) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return client.connection.BulkMetrics(handles)
}

func (client *client) Lookup(handle string) (Container, error) {
	handles, err := client.connection.List(nil)
	if err != nil {
		return nil, err
	}

	for _, h := range handles {
		if h == handle {
			return newContainer(handle, client.connection), nil
		}
	}

	return nil, garden.ContainerNotFoundError{Handle: handle}
}
