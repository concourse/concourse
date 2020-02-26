package libcontainerd

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Client
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/containerd/containerd.Container
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/containerd/containerd.Task
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/containerd/containerd.Process
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/containerd/containerd/cio.IO

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
	addr           string
	namespace      string
	requestTimeout time.Duration

	containerd *containerd.Client
}

var _ Client = (*client)(nil)

func New(addr, namespace string, requestTimeout time.Duration) *client {
	return &client{
		addr:           addr,
		namespace:      namespace,
		requestTimeout: requestTimeout,
	}
}

func (c *client) Init() (err error) {
	c.containerd, err = containerd.New(
		c.addr,
		containerd.WithDefaultNamespace(c.namespace),
	)
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
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

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
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	return c.containerd.Containers(ctx, labels...)
}

func (c *client) GetContainer(ctx context.Context, handle string) (containerd.Container, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	cont, err := c.containerd.LoadContainer(ctx, handle)
	if err != nil {
		return nil, err
	}

	return &container{
		requestTimeout: c.requestTimeout,
		container:      cont,
	}, nil
}

func (c *client) Version(ctx context.Context) (err error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	_, err = c.containerd.Version(ctx)
	return
}
func (c *client) Destroy(ctx context.Context, handle string) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	container, err := c.GetContainer(ctx, handle)
	if err != nil {
		return err
	}

	return container.Delete(ctx)
}
