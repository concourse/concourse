package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/concourse/concourse/worker/runtime/iptables"
	"github.com/containerd/containerd"
	"github.com/containerd/go-cni"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//counterfeiter:generate github.com/containerd/go-cni.CNI

// CNINetworkConfig provides configuration for CNINetwork to override the
// defaults.
type CNINetworkConfig struct {
	// BridgeName is the name that the bridge set up in the current network
	// namespace to connect the veth's to.
	//
	BridgeName string

	// NetworkName is the virtual name used to identify the managed network.
	//
	NetworkName string

	// MTU is the MTU of the bridge network interface.
	//
	MTU int

	// IPv4 Configuration
	//
	IPv4 CNIv4NetworkConfig

	// IPv6 Configuration
	//
	IPv6 CNIv6NetworkConfig
}

type CNIv4NetworkConfig struct {

	// The subnet (in CIDR notation) which the veths should be
	// added to.
	//
	Subnet string
}

type CNIv6NetworkConfig struct {
	// Enable IPv6 networking
	//
	Enabled bool

	// The subnet (in CIDR notation) which the veths should be
	// added to.
	//
	Subnet string

	// Masquerade the traffic from the container using the worker address
	//
	IPMasq bool
}

const (
	// networkMountsDir is a default directory used for storing
	// container-related files inside the worker's WorkDir
	//
	networkMountsDir = "networkmounts"

	ipTablesAdminChainName = "CONCOURSE-OPERATOR"
)

var (
	// DefaultCNINetworkConfig is the default configuration for the CNI network
	// created to put concourse containers into.
	//
	DefaultCNINetworkConfig = CNINetworkConfig{
		BridgeName:  "concourse0",
		NetworkName: "concourse",
		IPv4: CNIv4NetworkConfig{
			Subnet: "10.80.0.0/16",
		},
		IPv6: CNIv6NetworkConfig{
			Enabled: true,
			Subnet:  "fd9c:31a6:c759::/64",
			IPMasq:  true,
		},
	}
	// Default firewall plugin configuration
	//
	defaultFirewallPlugin = FirewallPlugin{
		Plugin:            Plugin{"firewall"},
		IPTablesChainName: ipTablesAdminChainName,
	}

	// Default IPv4 route
	//
	_, defaultRouteV4, _ = net.ParseCIDR("0.0.0.0/0")

	// Default IPv6 route
	//
	_, defaultRouteV6, _ = net.ParseCIDR("::/0")
)

type CNINetworkConfiguration struct {
	Name       string        `json:"name"`
	CNIVersion string        `json:"cniVersion"`
	Plugins    []interface{} `json:"plugins"`
}

type Plugin struct {
	Type string `json:"type"`
}

type BridgePlugin struct {
	Plugin
	Bridge    string `json:"bridge"`
	IsGateway bool   `json:"isGateway"`
	IPMasq    bool   `json:"ipMasq"`
	IPAM      IPAM   `json:"ipam"`
	MTU       int    `json:"mtu,omitempty"`
}

type FirewallPlugin struct {
	Plugin
	IPTablesChainName string `json:"iptablesAdminChainName"`
}

type IPAM struct {
	Type   string        `json:"type"`
	Ranges [][]Range     `json:"ranges"`
	Routes []types.Route `json:"routes"`
}

type Range struct {
	Subnet types.IPNet `json:"subnet"`
}

func (c CNINetworkConfig) ToJSONv4() string {
	_, subnet, err := net.ParseCIDR(c.IPv4.Subnet)
	if err != nil {
		_, subnet, _ = net.ParseCIDR(DefaultCNINetworkConfig.IPv4.Subnet)
	}

	ranges := [][]Range{
		{{Subnet: types.IPNet(*subnet)}},
	}

	routes := []types.Route{
		{Dst: *subnet},
		{Dst: *defaultRouteV4},
	}

	bridgePlugin := BridgePlugin{
		Plugin:    Plugin{"bridge"},
		Bridge:    c.BridgeName,
		IsGateway: true,
		IPMasq:    true,
		MTU:       c.MTU,
		IPAM: IPAM{
			Type:   "host-local",
			Ranges: ranges,
			Routes: routes,
		},
	}

	netConfig := CNINetworkConfiguration{
		Name:       c.NetworkName,
		CNIVersion: "0.4.0",
		Plugins: []interface{}{
			bridgePlugin,
			defaultFirewallPlugin,
		},
	}

	config, _ := json.Marshal(netConfig)

	return string(config)
}

func (c CNINetworkConfig) ToJSONv6() string {
	_, subnet, err := net.ParseCIDR(c.IPv6.Subnet)
	if err != nil {
		_, subnet, _ = net.ParseCIDR(DefaultCNINetworkConfig.IPv6.Subnet)
	}

	ranges := [][]Range{
		{{Subnet: types.IPNet(*subnet)}},
	}

	routes := []types.Route{
		{Dst: *subnet},
		{Dst: *defaultRouteV6},
	}

	bridgePlugin := BridgePlugin{
		Plugin:    Plugin{"bridge"},
		Bridge:    c.BridgeName,
		IsGateway: true,
		IPMasq:    c.IPv6.IPMasq,
		MTU:       c.MTU,
		IPAM: IPAM{
			Type:   "host-local",
			Ranges: ranges,
			Routes: routes,
		},
	}

	netConfig := CNINetworkConfiguration{
		Name:       c.NetworkName,
		CNIVersion: "0.4.0",
		Plugins: []interface{}{
			bridgePlugin,
			defaultFirewallPlugin,
		},
	}

	config, _ := json.Marshal(netConfig)

	return string(config)
}

// CNINetworkOpt defines a functional option that when applied, modifies the
// configuration of a CNINetwork.
type CNINetworkOpt func(n *cniNetwork)

// WithCNIBinariesDir is the directory where the binaries necessary for setting
// up the network live.
func WithCNIBinariesDir(dir string) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.binariesDir = dir
	}
}

// WithNameServers sets the set of nameservers to be configured for the
// /etc/resolv.conf inside the containers.
func WithNameServers(nameservers []string) CNINetworkOpt {
	return func(n *cniNetwork) {
		for _, ns := range nameservers {
			n.nameServers = append(n.nameServers, "nameserver "+ns)
		}
	}
}

// WithCNIClient is an implementor of the CNI interface for reaching out to CNI
// plugins.
func WithCNIClient(c cni.CNI) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.client = c
	}
}

// WithCNINetworkConfig provides a custom CNINetworkConfig to be used by the CNI
// client at startup time.
func WithCNINetworkConfig(c CNINetworkConfig) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.config = c
	}
}

// WithCNIFileStore changes the default FileStore used to store files that
// belong to network configurations for containers.
func WithCNIFileStore(f FileStore) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.store = f
	}
}

// FileStoreWithWorkDir creates a Filestore specific to the CNI networks
// working directory
func FileStoreWithWorkDir(path string) FileStore {
	return NewFileStore(filepath.Join(path, networkMountsDir))
}

// WithRestrictedNetworks defines the network ranges that containers will be restricted
// from accessing.
func WithRestrictedNetworks(restrictedNetworks []string) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.restrictedNetworks = restrictedNetworks
	}
}

// WithAllowHostAccess allows containers to talk to the host
func WithAllowHostAccess() CNINetworkOpt {
	return func(n *cniNetwork) {
		n.allowHostAccess = true
	}
}

// WithIptables allows for a custom implementation of the iptables.Iptables interface
// to be provided.
func WithIptables(ipt iptables.Iptables) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.ipt = ipt
	}
}

// WithDefaultsForTesting testing damage
func WithDefaultsForTesting() CNINetworkOpt {
	return func(n *cniNetwork) {
		if n.binariesDir == "" {
			n.binariesDir = "/usr/local/concourse/bin"
		}
		if n.store == nil {
			n.store = NewFileStore("/tmp")
		}
	}
}

type cniNetwork struct {
	client             cni.CNI
	store              FileStore
	config             CNINetworkConfig
	nameServers        []string
	binariesDir        string
	restrictedNetworks []string
	allowHostAccess    bool
	ipt                iptables.Iptables
}

var _ Network = (*cniNetwork)(nil)

func NewCNINetwork(opts ...CNINetworkOpt) (*cniNetwork, error) {
	var err error

	n := &cniNetwork{
		config: DefaultCNINetworkConfig,
	}

	for _, opt := range opts {
		opt(n)
	}

	if n.binariesDir == "" {
		return nil, fmt.Errorf("missing binaries dir")
	}

	if n.store == nil {
		return nil, fmt.Errorf("no file store initialized")
	}

	if n.client == nil {
		n.client, err = cni.New(cni.WithPluginDir([]string{n.binariesDir}))
		if err != nil {
			return nil, fmt.Errorf("cni init: %w", err)
		}

		opts := []cni.Opt{
			cni.WithConfListBytes([]byte(n.config.ToJSONv4())),
			cni.WithLoNetwork,
		}
		if n.config.IPv6.Enabled {
			opts = append(opts, cni.WithConfListBytes([]byte(n.config.ToJSONv6())))
		}

		err = n.client.Load(opts...)
		if err != nil {
			return nil, fmt.Errorf("cni configuration loading: %w", err)
		}
	}

	if n.ipt == nil {
		n.ipt, err = iptables.New()

		if err != nil {
			return nil, fmt.Errorf("failed to initialize iptables: %w", err)
		}
	}

	return n, nil
}

func (n cniNetwork) SetupHostNetwork() error {
	err := n.setupRestrictedNetworks()
	if err != nil {
		return err
	}

	if !n.allowHostAccess {
		err = n.restrictHostAccess()
		if err != nil {
			return err
		}
	}

	return nil
}

func (n cniNetwork) SetupMounts(handle string) ([]specs.Mount, error) {
	if handle == "" {
		return nil, ErrInvalidInput("empty handle")
	}

	etcHosts, err := n.store.Create(
		filepath.Join(handle, "/hosts"),
		[]byte("127.0.0.1 localhost\n"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating /etc/hosts: %w", err)
	}

	etcHostName, err := n.store.Create(
		filepath.Join(handle, "/hostname"),
		[]byte(handle+"\n"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating /etc/hostname: %w", err)
	}

	resolvContents, err := n.generateResolvConfContents()
	if err != nil {
		return nil, fmt.Errorf("generating resolv.conf: %w", err)
	}

	resolvConf, err := n.store.Create(
		filepath.Join(handle, "/resolv.conf"),
		resolvContents,
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
			Destination: "/etc/hostname",
			Type:        "bind",
			Source:      etcHostName,
			Options:     []string{"bind", "rw"},
		}, {
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      resolvConf,
			Options:     []string{"bind", "rw"},
		},
	}, nil
}

const filterTable = "filter"

func (n cniNetwork) setupRestrictedNetworks() error {
	err := n.ipt.CreateChainOrFlushIfExists(filterTable, ipTablesAdminChainName)
	if err != nil {
		return fmt.Errorf("create chain or flush if exists failed: %w", err)
	}

	// Optimization that allows packets of ESTABLISHED and RELATED connections to go through without further rule matching
	err = n.ipt.AppendRule(filterTable, ipTablesAdminChainName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	if err != nil {
		return fmt.Errorf("appending accept rule for RELATED & ESTABLISHED connections failed: %w", err)
	}

	for _, restrictedNetwork := range n.restrictedNetworks {
		// Create REJECT rule in admin chain
		err = n.ipt.AppendRule(filterTable, ipTablesAdminChainName, "-d", restrictedNetwork, "-j", "REJECT")
		if err != nil {
			return fmt.Errorf("appending reject rule for restricted network %s failed: %w", restrictedNetwork, err)
		}
	}
	return nil
}

func (n cniNetwork) generateResolvConfContents() ([]byte, error) {
	contents := ""
	resolvConfEntries := n.nameServers
	var err error

	if len(n.nameServers) == 0 {
		resolvConfEntries, err = ParseHostResolveConf("/etc/resolv.conf")
	}

	contents = strings.Join(resolvConfEntries, "\n") + "\n"

	return []byte(contents), err
}

func (n cniNetwork) restrictHostAccess() error {
	err := n.ipt.CreateChainOrFlushIfExists(filterTable, "INPUT")
	if err != nil {
		return fmt.Errorf("create chain or flush if exists failed: %w", err)
	}

	err = n.ipt.AppendRule(filterTable, "INPUT", "-i", n.config.BridgeName, "-j", "REJECT", "--reject-with", "icmp-host-prohibited")
	if err != nil {
		return fmt.Errorf("error appending iptables rule: %w", err)
	}

	return nil
}

func (n cniNetwork) DropContainerTraffic(containerHandle string) error {
	containerIp, err := n.store.ContainerIpLookup(containerHandle)
	if err != nil {
		return fmt.Errorf("error getting container IP: %w", err)
	}

	err = n.ipt.InsertRule(filterTable, "INPUT", 1, "-s", containerIp, "-j", "DROP")
	if err != nil {
		return fmt.Errorf("error inserting iptables rule to INPUT: %w", err)
	}

	err = n.ipt.InsertRule(filterTable, "FORWARD", 1, "-s", containerIp, "-j", "DROP")
	if err != nil {
		return fmt.Errorf("error inserting iptables rule to FORWARD: %w", err)
	}

	return nil
}

func (n cniNetwork) ResumeContainerTraffic(containerHandle string) error {
	containerIp, err := n.store.ContainerIpLookup(containerHandle)
	if err != nil {
		return fmt.Errorf("error getting container IP: %w", err)
	}

	err = n.ipt.DeleteRule(filterTable, "INPUT", "-s", containerIp, "-j", "DROP")
	if err != nil {
		return fmt.Errorf("error deleting iptables rule in INPUT: %w", err)
	}

	err = n.ipt.DeleteRule(filterTable, "FORWARD", "-s", containerIp, "-j", "DROP")
	if err != nil {
		return fmt.Errorf("error deleting iptables rule in FORWARD: %w", err)
	}

	return nil
}

func (n cniNetwork) Add(ctx context.Context, task containerd.Task, containerHandle string) error {
	if task == nil {
		return ErrInvalidInput("nil task")
	}

	id, netns := netId(task), netNsPath(task)

	result, err := n.client.Setup(ctx, id, netns)

	if err != nil {
		return fmt.Errorf("cni net setup: %w", err)
	}

	// Find container IP
	config, found := result.Interfaces["eth0"]
	if !found || len(config.IPConfigs) == 0 {
		return fmt.Errorf("cni net setup: no eth0 interface found")
	}

	// Update /etc/hosts on container
	// This could not be done earlier because we only have the container IP after the network has been setup
	return n.store.Append(
		filepath.Join(containerHandle, "/hosts"),
		[]byte(config.IPConfigs[0].IP.String()+" "+containerHandle+"\n"),
	)
}

func (n cniNetwork) Remove(ctx context.Context, task containerd.Task, handle string) error {
	var err error
	if task == nil {
		return ErrInvalidInput("nil task")
	}

	id, netns := netId(task), netNsPath(task)

	err = n.store.Delete(handle)
	if err != nil {
		return fmt.Errorf("cni network mounts teardown: %w", err)
	}

	err = n.client.Remove(ctx, id, netns)
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
