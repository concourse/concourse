package runtime

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//counterfeiter:generate . Network
type Network interface {
	// SetupMounts prepares mounts that might be necessary for proper
	// networking functionality.
	//
	SetupMounts(handle string) (mounts []specs.Mount, err error)

	// SetupRestrictedNetworks sets up networking rules to prevent
	// container access to specified network ranges
	//
	SetupRestrictedNetworks() (err error)

	// Add adds a task to the network.
	//
	Add(ctx context.Context, task containerd.Task) (err error)

	// Removes a task from the network.
	//
	Remove(ctx context.Context, task containerd.Task) (err error)
}
