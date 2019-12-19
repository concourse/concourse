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
	clientTimeout time.Duration
}

type InputValidationError struct{
	Message string
}

func (e InputValidationError) Error() string {
	return "input validation error: " + e.Message
}
type ClientError struct {
	InnerError error
}

func (e ClientError) Error() string {
	return "client error: " + e.InnerError.Error()
}

func New(client libcontainerd.Client, namespace string) Backend {
	return Backend{
		namespace: namespace,
		client:    client,
		clientTimeout: 10 * time.Second,
	}
}

func NewWithTimeout(client libcontainerd.Client, namespace string, clientTimeout time.Duration) Backend {
	return Backend{
		namespace: namespace,
		client:    client,
		clientTimeout: clientTimeout,
	}
}

// Start initializes the client.
//
func (b *Backend) Start() (err error) {
	err = b.client.Init()
	if err != nil {
		return ClientError{ InnerError: fmt.Errorf("failed to initialize containerd client: %w", err) }
	}

	return
}

// Stop closes the client's underlying connections and frees any resources
// associated with it.
//
func (b *Backend) Stop() {
	_ = b.client.Stop()
}

// GraceTime is the maximum duration that a container can stick around for,
// after there no references to a container in the client using Garden; in our case,
// this means when the ATC DB has no record of that container anymore.
//
func (b *Backend) GraceTime(container garden.Container) (duration time.Duration) {
	return
}

// Pings the garden server in order to check connectivity.
//
func (b *Backend) Ping() (err error) {
	err = b.client.Version(context.Background())
	if err != nil {
		return ClientError{ InnerError: err }
	}
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
	ctxWithTimeout, _ := context.WithTimeout(ctx, b.clientTimeout)

	oci, err = bespec.OciSpec(gdnSpec)
	if err != nil {
		err = ClientError{ InnerError: fmt.Errorf("failed to convert garden spec to oci spec: %w", err) }
		return
	}

	cont, err := b.client.NewContainer(ctxWithTimeout,
		gdnSpec.Handle, gdnSpec.Properties, oci,
	)

	if err != nil {
		err = ClientError{ InnerError: fmt.Errorf("failed to create a container in containerd: %w", err) }
		return
	}

	_, err = cont.NewTask(ctxWithTimeout, cio.NullIO)
	if err != nil {
		err = ClientError{ InnerError: fmt.Errorf("failed to create a task in container: %w", err) }
		return
	}

	container = &Container{
		handle: cont.ID(),
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
// * Container handle does not exist
// * Container task was not successfully spun down prior to container deletion.
//   Reasons include task not found, termination signal failed, or task deletion failed.
// * Destroy request to client failed
//
func (b *Backend) Destroy(handle string) error {
	if handle == "" {
		return InputValidationError{Message: "handle is empty"}
	}

	ctx := namespaces.WithNamespace(context.Background(), b.namespace)
	ctxWithTimeout, _ := context.WithTimeout(ctx, b.clientTimeout)

	container, err := b.client.GetContainer(ctxWithTimeout, handle)
	if err != nil {
		return ClientError{ InnerError: err }
	}

	task, err := container.Task(ctxWithTimeout, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return ClientError { InnerError: err }
		}
	} else {
		err = killTasks(ctxWithTimeout, task)
		if err != nil {
			return ClientError{ InnerError: err }
		}
	}

	err = b.client.Destroy(ctxWithTimeout, handle)
	if err != nil {
		return ClientError{ InnerError: err }
	}
	return nil
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
	case status := <-exitStatus:
		if status.Error() != nil {
			return status.Error()
		}
	case <-ctx.Done():
		err = task.Kill(ctx, syscall.SIGKILL)
		if err != nil {
			return err
		}
	}

	result, err := task.Delete(ctx)
	if err != nil {
		return err
	}
	if result != nil && result.Error() != nil {
		return result.Error()
	}

	return nil
}

// Containers lists all containers filtered by Properties (which are ANDed together).
//
// Errors:
// * Problems communicating with containerd client
func (b *Backend) Containers(properties garden.Properties) (containers []garden.Container, err error) {
	ctx := namespaces.WithNamespace(context.Background(), b.namespace)
	ctxWithTimeout, _ := context.WithTimeout(ctx, b.clientTimeout)

	filters, err := propertiesToFilterList(properties)
	if err != nil {
		err = ClientError{ InnerError: err }
		return
	}

	res, err := b.client.Containers(ctxWithTimeout, filters...)
	if err != nil {
		err = ClientError{ InnerError: err }
		return
	}

	containers = make([]garden.Container, len(res))
	for i, containerdContainer := range res {
		gContainer := Container{
			handle: containerdContainer.ID(),
		}
		containers[i] = &gContainer
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
	ctxWithTimeout, _ := context.WithTimeout(ctx, b.clientTimeout)

	if handle == "" {
		return nil, InputValidationError{Message: "handle is empty"}
	}

	containerdContainer, err := b.client.GetContainer(ctxWithTimeout, handle)
	if err != nil {
		return nil, ClientError{ InnerError: err }
	}

	return &Container{handle: containerdContainer.ID()}, nil
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
