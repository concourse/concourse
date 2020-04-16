package backend

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/containerd/go-cni.CNI

// CNINetworkConfig provides configuration for CNINetwork to override the
// defaults.
//
type CNINetworkConfig struct {
	// BridgeName is the name that the bridge set up in the current network
	// namespace to connect the veth's to.
	//
	BridgeName string

	// NetworkName is the virtual name used to identify the managed network.
	//
	NetworkName string

	// Subnet is the subnet (in CIDR notation) which the veths should be
	// added to.
	//
	Subnet string
}

const (
	// fileStoreWorkDir is a default directory used for storing
	// container-related files
	//
	fileStoreWorkDir = "/tmp"

	// binariesDir corresponds to the directory where CNI plugins have their
	// binaries in.
	//
	binariesDir = "/usr/local/concourse/bin"
)

var (
	// defaultNameServers is the default set of nameservers used.
	//
	defaultNameServers = []string{"8.8.8.8"}

	// defaultCNINetworkConfig is the default configuration for the CNI network
	// created to put concourse containers into.
	//
	defaultCNINetworkConfig = CNINetworkConfig{
		BridgeName:  "concourse0",
		NetworkName: "concourse",
		Subnet:      "10.80.0.0/16",
	}
)

func (c CNINetworkConfig) ToJSON() string {
	const networksConfListFormat = `{
  "cniVersion": "0.4.0",
  "name": "%s",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "%s",
      "isGateway": true,
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "%s",
        "routes": [
          {
            "dst": "0.0.0.0/0"
          }
        ]
      }
    },
    {
      "type": "firewall"
    }
  ]
}`

	return fmt.Sprintf(networksConfListFormat,
		c.NetworkName, c.BridgeName, c.Subnet,
	)
}

// CNINetworkOpt defines a functional option that when applied, modifies the
// configuration of a CNINetwork.
//
type CNINetworkOpt func(n *cniNetwork)

// WithCNIBinariesDir is the directory where the binaries necessary for setting
// up the network live.
//
func WithCNIBinariesDir(dir string) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.binariesDir = dir
	}
}

// WithNameServers sets the set of nameservers to be configured for the
// /etc/resolv.conf inside the containers.
//
func WithNameServers(nameservers []string) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.nameServers = nameservers
	}
}

// WithCNIClient is an implementor of the CNI interface for reaching out to CNI
// plugins.
//
func WithCNIClient(c cni.CNI) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.client = c
	}
}

// WithCNINetworkConfig provides a custom CNINetworkConfig to be used by the CNI
// client at startup time.
//
func WithCNINetworkConfig(c CNINetworkConfig) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.config = c
	}
}

// WithCNIFileStore changes the default FileStore used to store files that
// belong to network configurations for containers.
//
func WithCNIFileStore(f FileStore) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.store = f
	}
}

type cniNetwork struct {
	client      cni.CNI
	store       FileStore
	config      CNINetworkConfig
	nameServers []string
	binariesDir string
}

var _ Network = (*cniNetwork)(nil)

func NewCNINetwork(opts ...CNINetworkOpt) (*cniNetwork, error) {
	var err error

	n := &cniNetwork{
		binariesDir: binariesDir,
		config:      defaultCNINetworkConfig,
		nameServers: defaultNameServers,
	}

	for _, opt := range opts {
		opt(n)
	}

	if n.store == nil {
		n.store = NewFileStore(fileStoreWorkDir)
	}

	if n.client == nil {
		n.client, err = cni.New(cni.WithPluginDir([]string{n.binariesDir}))
		if err != nil {
			return nil, fmt.Errorf("cni init: %w", err)
		}

		err = n.client.Load(
			cni.WithConfListBytes([]byte(n.config.ToJSON())),
			cni.WithLoNetwork,
		)
		if err != nil {
			return nil, fmt.Errorf("cni configuration loading: %w", err)
		}
	}

	return n, nil
}

func (n cniNetwork) SetupMounts(handle string) ([]specs.Mount, error) {
	if handle == "" {
		return nil, ErrInvalidInput("empty handle")
	}

	etcHosts, err := n.store.Create(
		filepath.Join(handle, "/hosts"),
		[]byte("127.0.0.1 localhost"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating /etc/hosts: %w", err)
	}

	resolvConf, err := n.store.Create(
		filepath.Join(handle, "/resolv.conf"),
		n.generateResolvConfContents(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating /etc/resolv.conf: %w", err)
	}

	return []specs.Mount{
		{
			Destination: "/etc/hosts",
			Type:        "bind",
			Source:      etcHosts,
			Options:     []string{"bind", "rw"},
		}, {
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      resolvConf,
			Options:     []string{"bind", "rw"},
		},
	}, nil
}

func (n cniNetwork) generateResolvConfContents() []byte {
	contents := ""
	for _, n := range n.nameServers {
		contents = contents + fmt.Sprintf("nameserver %s\n", n)
	}

	return []byte(contents)
}

func (n cniNetwork) Add(ctx context.Context, task containerd.Task) error {
	if task == nil {
		return ErrInvalidInput("nil task")
	}

	id, netns := netId(task), netNsPath(task)

	_, err := n.client.Setup(ctx, id, netns)
	if err != nil {
		return fmt.Errorf("cni net setup: %w", err)
	}

	return nil
}

func (n cniNetwork) Remove(ctx context.Context, task containerd.Task) error {
	if task == nil {
		return ErrInvalidInput("nil task")
	}

	id, netns := netId(task), netNsPath(task)

	err := n.client.Remove(ctx, id, netns)
	if err != nil {
		return fmt.Errorf("cni net teardown: %w", err)
	}

	return nil
}

func netId(task containerd.Task) string {
	return task.ID()
}

func netNsPath(task containerd.Task) string {
	return fmt.Sprintf("/proc/%d/ns/net", task.Pid())
}
