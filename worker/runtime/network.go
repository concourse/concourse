package runtime

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//counterfeiter:generate . Network
type Network interface {
	// SetupHostNetwork sets up networking rules that
	// affect all containers
	//
	SetupHostNetwork() (err error)

	// SetupMounts prepares mounts that might be necessary for proper
	// networking functionality.
	//
	SetupMounts(handle string) (mounts []specs.Mount, err error)

	// Add adds a task to the network.
	//
	Add(ctx context.Context, task containerd.Task, containerHandle string) (err error)

	// Removes a task from the network.
	//
	Remove(ctx context.Context, task containerd.Task, handle string) (err error)
}
