package libcontainerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Client
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/containerd/containerd.Container
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/containerd/containerd.Task

// Client represents the minimum interface used to communicate with containerd
// to manage containers.
//
type Client interface {

	// Init provides the initialization of internal structures necessary by
	// the client, e.g., instantiation of the gRPC client.
	//
	Init() (err error)

	// Version queries containerd's version service in order to verify
	// connectivity.
	//
	Version(ctx context.Context) (err error)

	// Stop deallocates any initialization performed by `Init()` and
	// subsequent calls to methods of this interface.
	//
	Stop() (err error)

	// NewContainer creates a container in containerd.
	//
	NewContainer(
		ctx context.Context,
		id string,
		labels map[string]string,
		oci *specs.Spec,
	) (
		container containerd.Container, err error,
	)

	// Containers lists containers available in containerd matching a given
	// labelset.
	//
	Containers(
		ctx context.Context,
		labels ...string,
	) (
		containers []containerd.Container, err error,
	)

	// GetContainer retrieves a created container that matches the specified handle.
	//
	GetContainer(
		ctx context.Context,
		handle string,
	) (
		container containerd.Container, err error,
	)

	// Destroy stops any running tasks on a container and removes the container.
	// If a task cannot be stopped gracefully, it will be forcefully stopped after
	// a timeout period (default 10 seconds).
	//
	Destroy(ctx context.Context, handle string) error
}

type client struct {
	addr string

	containerd *containerd.Client
}

var _ Client = (*client)(nil)

func New(addr string) *client {
	return &client{addr: addr}
}

func (c *client) Init() (err error) {
	c.containerd, err = containerd.New(c.addr)
	if err != nil {
		err = fmt.Errorf("failed to connect to addr %s: %w", c.addr, err)
		return
	}

	return
}

func (c *client) Stop() (err error) {
	if c.containerd == nil {
		return
	}

	err = c.containerd.Close()
	return
}

func (c *client) NewContainer(
	ctx context.Context, id string, labels map[string]string, oci *specs.Spec,
) (
	containerd.Container, error,
) {
	return c.containerd.NewContainer(ctx, id,
		containerd.WithSpec(oci),
		containerd.WithContainerLabels(labels),
	)
}

func (c *client) Containers(
	ctx context.Context, labels ...string,
) (
	[]containerd.Container, error,
) {
	return c.containerd.Containers(ctx, labels...)
}

func (c *client) GetContainer(ctx context.Context, handle string) (containerd.Container, error) {
	return c.containerd.LoadContainer(ctx, handle)
}

func (c *client) Version(ctx context.Context) (err error) {
	_, err = c.containerd.Version(ctx)
	return
}
func (c *client) Destroy(ctx context.Context, handle string) error {
	container, err := c.GetContainer(ctx, handle)
	if err != nil {
		return err
	}

	return container.Delete(ctx)
}
