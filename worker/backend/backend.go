package backend

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend/libcontainerd"
	bespec "github.com/concourse/concourse/worker/backend/spec"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ garden.Backend = (*Backend)(nil)

type Backend struct {
	client    libcontainerd.Client
	namespace string
}

type InputValidationError struct{}

func (e InputValidationError) Error() string {
	return "input validation error"
}

func New(client libcontainerd.Client, namespace string) Backend {
	return Backend{
		namespace: namespace,
		client:    client,
	}
}

// Start initializes the client.
//
func (b *Backend) Start() (err error) {
	err = b.client.Init()
	if err != nil {
		err = fmt.Errorf("failed to initialize contianerd client: %w", err)
		return
	}

	return
}

// Stop closes the client's underlying connections and frees any resources
// associated with it.
//
func (b *Backend) Stop() {
	_ = b.client.Stop()
}

func (b *Backend) GraceTime(container garden.Container) (duration time.Duration) {
	return
}

// Pings the garden server in order to check connectivity.
//
func (b *Backend) Ping() (err error) {
	err = b.client.Version(context.Background())
	return
}

// Capacity returns the physical capacity of the server's machine.
//
// Errors:
// * None.
func (b *Backend) Capacity() (capacity garden.Capacity, err error) { return }

// Create creates a new container.
//
func (b *Backend) Create(gdnSpec garden.ContainerSpec) (container garden.Container, err error) {
	var oci *specs.Spec
	ctx := namespaces.WithNamespace(context.Background(), b.namespace)

	oci, err = bespec.OciSpec(gdnSpec)
	if err != nil {
		err = fmt.Errorf("failed to convert garden spec to oci spec: %w", err)
		return
	}

	cont, err := b.client.NewContainer(ctx,
		gdnSpec.Handle, gdnSpec.Properties, oci,
	)
	if err != nil {
		err = fmt.Errorf("failed to create a container in containerd: %w", err)
		return
	}

	_, err = cont.NewTask(ctx, cio.NullIO)
	if err != nil {
		err = fmt.Errorf("failed to create a task in container: %w", err)
		return
	}

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
func (b *Backend) Destroy(handle string) error {
	ctx := namespaces.WithNamespace(context.Background(), b.namespace)
	const maxTaskKillWaitTime = 10 * time.Second

	if handle == "" {
		return InputValidationError{}
	}

	container, err := b.client.GetContainer(ctx, handle)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}

		return b.client.Destroy(ctx, handle)
	}

	timeDelimitedContext, _ := context.WithTimeout(ctx, maxTaskKillWaitTime)
	err = killTasks(timeDelimitedContext, task)
	if err != nil {
		return err
	}

	return b.client.Destroy(ctx, handle)
}

// killTasks kills a task on time, gracefully if possible, ungracefully if not.
//
func killTasks(ctx context.Context, task containerd.Task) error {
	exitStatus, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	err = task.Kill(ctx, syscall.SIGTERM)
	if err != nil {
		return err
	}

	select {
	case <-exitStatus:
		_, err = task.Delete(ctx) // todo: we're swallowing exitcodes in both these forks, do we care?
		return err
	case <-ctx.Done():
		err = task.Kill(ctx, syscall.SIGKILL) // should return GRPC DeadlineExceeded error type, wrapped up
		if err != nil {
			return err
		}
		_, err = task.Delete(ctx)
	}

	if err != nil {
		return err
	}

	return nil
}

// Containers lists all containers filtered by Properties (which are ANDed together).
//
// Errors:
// * None.
func (b *Backend) Containers(properties garden.Properties) (containers []garden.Container, err error) {
	ctx := namespaces.WithNamespace(context.Background(), b.namespace)

	filters, err := propertiesToFilterList(properties)
	if err != nil {
		return
	}

	res, err := b.client.Containers(ctx, filters...)
	if err != nil {
		return
	}

	containers = make([]garden.Container, len(res))
	for idx := range res {
		gContainer := NewContainer()
		containers[idx] = &gContainer
	}

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
func (b *Backend) Lookup(handle string) (garden.Container, error) {
	ctx := namespaces.WithNamespace(context.Background(), b.namespace)

	if handle == "" {
		return nil, InputValidationError{}
	}

	_, err := b.client.GetContainer(ctx, handle)
	if err != nil {
		return nil, err
	}

	return &Container{}, nil
}

// propertiesToFilterList converts a set of garden properties to a list of
// filters as expected by containerd.
//
func propertiesToFilterList(properties garden.Properties) (filters []string, err error) {
	filters = make([]string, len(properties))

	idx := 0
	for k, v := range properties {
		if k == "" || v == "" {
			err = fmt.Errorf("key or value must not be empty")
			return
		}

		filters[idx] = k + "=" + v
		idx++
	}

	return
}
