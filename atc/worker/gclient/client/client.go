package client

import (
	"context"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/atc/worker/gclient/client/connection"
)

type Client interface {
	gclient.Client
}

type client struct {
	connection connection.Connection
}

func New(connection connection.Connection) gclient.Client {
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

func (client *client) Create(ctx context.Context, spec garden.ContainerSpec) (gclient.Container, error) {
	handle, err := client.connection.Create(ctx, spec)
	if err != nil {
		return nil, err
	}

	return newContainer(handle, client.connection), nil
}

func (client *client) Containers(properties garden.Properties) ([]gclient.Container, error) {
	handles, err := client.connection.List(properties)
	if err != nil {
		return nil, err
	}

	containers := []gclient.Container{}
	for _, handle := range handles {
		containers = append(containers, newContainer(handle, client.connection))
	}

	return containers, nil
}

func (client *client) Destroy(ctx context.Context, handle string) error {
	err := client.connection.Destroy(ctx, handle)

	return err
}

func (client *client) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return client.connection.BulkInfo(handles)
}

func (client *client) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return client.connection.BulkMetrics(handles)
}

func (client *client) Lookup(handle string) (gclient.Container, error) {
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
