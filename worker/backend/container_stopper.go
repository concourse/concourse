package backend

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ContainerStopper

type ContainerStopper interface {
	// GracefullyStop stops a container giving the running processes a
	// chance to terminate.
	//
	GracefullyStop(ctx context.Context, container containerd.Container) (err error)

	// UngracefullyStop ungracefully terminates running processes on a
	// container, giving it no chance to gracefully finish its work.
	//
	UngracefullyStop(ctx context.Context, container containerd.Container) (err error)
}

func NewContainerStopper(
	gracefulKiller Killer,
	ungracefulKiller Killer,
) *containerStopper {
	return &containerStopper{
		gracefulKiller:   gracefulKiller,
		ungracefulKiller: ungracefulKiller,
	}
}

type containerStopper struct {
	gracefulKiller   Killer
	ungracefulKiller Killer
}

func (c containerStopper) GracefullyStop(
	ctx context.Context,
	container containerd.Container,
) error {
	return Stop(ctx, container, c.gracefulKiller)
}

func (c containerStopper) UngracefullyStop(
	ctx context.Context,
	container containerd.Container,
) error {
	return Stop(ctx, container, c.ungracefulKiller)
}

func Stop(
	ctx context.Context,
	container containerd.Container,
	killer Killer,
) error {
	task, err := container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("container's task retrieval: %w", err)
		}

		return nil
	}

	err = killer.Kill(context.Background(), task)
	if err != nil {
		return fmt.Errorf("killer kill: %w", err)
	}

	_, err = task.Delete(ctx)
	if err != nil {
		return fmt.Errorf("task deletion: %w", err)
	}

	return nil
}
