//go:build windows

package runtime

import (
	"context"
	"fmt"

	"github.com/Microsoft/hcsshim/hcn"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// DefaultHCNNetworkName is the name of the HNS network containers get
	// attached to. "nat" is the default network created by the Windows
	// containers feature.
	DefaultHCNNetworkName = "nat"

	// DefaultHCNNetworkPool is the subnet used when the configured HNS
	// network doesn't exist yet and has to be created. It matches the
	// default network pool of the Linux containerd runtime.
	DefaultHCNNetworkPool = "10.80.0.0/24"

	DefaultHCNNetworkGateway = "10.80.0.1"
)

// ContainerAttacher is implemented by networks that wire containers up at
// creation time (before the container starts) rather than joining a running
// task to the network.
type ContainerAttacher interface {
	AttachContainer(handle string, oci *specs.Spec) error
}

// HCNNetwork attaches containers to a Windows Host Compute Network,
// providing NAT'd outbound connectivity. It is the Windows-native
// equivalent of the CNI nat plugin, driven through the same HNS APIs.
type HCNNetwork struct {
	networkName string
	pool        string
	gateway     string
}

var (
	_ Network           = (*HCNNetwork)(nil)
	_ ContainerAttacher = (*HCNNetwork)(nil)
)

type HCNNetworkOpt func(n *HCNNetwork)

// WithHCNNetworkName configures the name of the HNS network containers are
// attached to.
func WithHCNNetworkName(name string) HCNNetworkOpt {
	return func(n *HCNNetwork) {
		n.networkName = name
	}
}

// WithHCNNetworkPool configures the subnet (and its gateway) used when the
// HNS network has to be created.
func WithHCNNetworkPool(pool, gateway string) HCNNetworkOpt {
	return func(n *HCNNetwork) {
		n.pool = pool
		n.gateway = gateway
	}
}

func NewHCNNetwork(opts ...HCNNetworkOpt) *HCNNetwork {
	n := &HCNNetwork{
		networkName: DefaultHCNNetworkName,
		pool:        DefaultHCNNetworkPool,
		gateway:     DefaultHCNNetworkGateway,
	}

	for _, opt := range opts {
		opt(n)
	}

	return n
}

// SetupHostNetwork ensures the HNS network exists, creating a NAT network
// when it doesn't.
func (n *HCNNetwork) SetupHostNetwork() error {
	_, err := hcn.GetNetworkByName(n.networkName)
	if err == nil {
		return nil
	}

	if !hcn.IsNotFoundError(err) {
		return fmt.Errorf("hcn network %s lookup: %w", n.networkName, err)
	}

	network := &hcn.HostComputeNetwork{
		Name: n.networkName,
		Type: hcn.NAT,
		Ipams: []hcn.Ipam{{
			Type: "Static",
			Subnets: []hcn.Subnet{{
				IpAddressPrefix: n.pool,
				Routes: []hcn.Route{{
					NextHop:           n.gateway,
					DestinationPrefix: "0.0.0.0/0",
				}},
			}},
		}},
		SchemaVersion: hcn.V2SchemaVersion(),
	}

	_, err = network.Create()
	if err != nil {
		return fmt.Errorf("create hcn network %s: %w", n.networkName, err)
	}

	return nil
}

// AttachContainer creates a network namespace and an endpoint on the HNS
// network for the container, and points the OCI spec at the namespace so the
// runtime attaches the endpoint when the container starts.
func (n *HCNNetwork) AttachContainer(handle string, oci *specs.Spec) error {
	network, err := hcn.GetNetworkByName(n.networkName)
	if err != nil {
		return fmt.Errorf("hcn network %s lookup: %w", n.networkName, err)
	}

	namespace, err := (&hcn.HostComputeNamespace{}).Create()
	if err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}

	endpoint := &hcn.HostComputeEndpoint{
		Name:               endpointName(handle),
		HostComputeNetwork: network.Id,
		SchemaVersion:      hcn.V2SchemaVersion(),
	}

	endpoint, err = endpoint.Create()
	if err != nil {
		_ = namespace.Delete()
		return fmt.Errorf("create endpoint: %w", err)
	}

	err = hcn.AddNamespaceEndpoint(namespace.Id, endpoint.Id)
	if err != nil {
		_ = endpoint.Delete()
		_ = namespace.Delete()
		return fmt.Errorf("add endpoint to namespace: %w", err)
	}

	if oci.Windows == nil {
		oci.Windows = &specs.Windows{}
	}
	oci.Windows.Network = &specs.WindowsNetwork{
		NetworkNamespace: namespace.Id,
	}

	return nil
}

func (n *HCNNetwork) SetupMounts(handle string, hermetic bool) ([]specs.Mount, error) {
	return nil, nil
}

// Add is a no-op: the container is attached to the network before it is
// created, in AttachContainer.
func (n *HCNNetwork) Add(ctx context.Context, task containerd.Task, containerHandle string) error {
	return nil
}

// Remove tears down the endpoint and namespace created for the container.
func (n *HCNNetwork) Remove(ctx context.Context, task containerd.Task, handle string) error {
	endpoint, err := hcn.GetEndpointByName(endpointName(handle))
	if err != nil {
		if hcn.IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("get endpoint: %w", err)
	}

	namespaceID := endpoint.HostComputeNamespace

	err = endpoint.Delete()
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}

	if namespaceID != "" {
		namespace, err := hcn.GetNamespaceByID(namespaceID)
		if err != nil {
			if hcn.IsNotFoundError(err) {
				return nil
			}
			return fmt.Errorf("get namespace: %w", err)
		}

		err = namespace.Delete()
		if err != nil {
			return fmt.Errorf("delete namespace: %w", err)
		}
	}

	return nil
}

// DropContainerTraffic is not supported on Windows yet; hermetic containers
// keep their network access.
func (n *HCNNetwork) DropContainerTraffic(containerHandle string) error {
	return nil
}

func (n *HCNNetwork) ResumeContainerTraffic(containerHandle string) error {
	return nil
}

func endpointName(handle string) string {
	return "concourse-" + handle
}
