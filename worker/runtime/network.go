package runtime

import (
	"context"
	"net"

	"github.com/containerd/containerd"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Network

type Network interface {
	// SetupMounts prepares mounts that might be necessary for proper
	// networking functionality.
	//
	SetupMounts(handle string) ([]specs.Mount, error)

	// SetupRestrictedNetworks sets up networking rules to prevent
	// container access to specified network ranges
	//
	SetupRestrictedNetworks() error

	// Add adds a task to the network and returns info about the
	// assigned IP addresses of the virtual ethernet pair.
	//
	Add(ctx context.Context, task containerd.Task) (*IPInfo, error)

	// Removes a task from the network.
	//
	Remove(ctx context.Context, task containerd.Task) error
}

type IPInfo struct {
	// ContainerIP is the IP address of the container side of the
	// virtual ethernet pair.
	ContainerIP net.IP

	// HostIP is the IP address of the gateway which controls the
	// host side of the virtual ethernet pair.
	HostIP net.IP
}
