package libcontainerd

import (
	"context"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	prototypes "github.com/gogo/protobuf/types"
)

var _ containerd.Container = (*container)(nil)

type container struct {
	requestTimeout time.Duration
	container      containerd.Container
}

func (c *container) ID() string {
	return c.container.ID()
}

func (c *container) Info(ctx context.Context, opts ...containerd.InfoOpts) (containers.Container, error) {
	return c.container.Info(ctx, opts...)
}

func (c *container) Delete(ctx context.Context, opts ...containerd.DeleteOpts) error {
	return c.container.Delete(ctx, opts...)
}

func (c *container) NewTask(ctx context.Context, creator cio.Creator, opts ...containerd.NewTaskOpts) (containerd.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	return c.container.NewTask(ctx, creator, opts...)
}

func (c *container) Spec(ctx context.Context) (*oci.Spec, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	return c.container.Spec(ctx)
}

func (c *container) Task(ctx context.Context, opt cio.Attach) (containerd.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	return c.container.Task(ctx, opt)
}

func (c *container) Image(ctx context.Context) (containerd.Image, error) {
	return c.container.Image(ctx)
}

func (c *container) Labels(ctx context.Context) (map[string]string, error) {
	return c.container.Labels(ctx)
}

func (c *container) SetLabels(ctx context.Context, labels map[string]string) (map[string]string, error) {
	return c.container.SetLabels(ctx, labels)

}

func (c *container) Extensions(ctx context.Context) (map[string]prototypes.Any, error) {
	return c.container.Extensions(ctx)

}

func (c *container) Update(ctx context.Context, opts ...containerd.UpdateContainerOpts) error {
	return c.container.Update(ctx, opts...)

}

func (c *container) Checkpoint(ctx context.Context, id string, opts ...containerd.CheckpointOpts) (containerd.Image, error) {
	return c.container.Checkpoint(ctx, id, opts...)
}
