//go:build linux

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/concourse/concourse/v8/worker/runtime/iptables"
	"github.com/concourse/concourse/v8/worker/runtime/nftables"
	containerd "github.com/containerd/containerd/v2/client"
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

	// FirewallBackendAuto uses nftables when the worker can create its table,
	// and otherwise keeps the iptables rules.
	FirewallBackendAuto = "auto"

	// FirewallBackendIPTables is the CNI firewall plugin plus the worker's
	// iptables admin chain.
	FirewallBackendIPTables = "iptables"

	// FirewallBackendNFTables writes the worker policy with nftables and leaves
	// the CNI firewall plugin out of the network config. The bridge plugin
	// masquerades through nftables on CNI plugins >= v1.6.0.
	FirewallBackendNFTables = "nftables"

	ipMasqBackendNFTables = "nftables"
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
	Name       string `json:"name"`
	CNIVersion string `json:"cniVersion"`
	Plugins    []any  `json:"plugins"`
}

type Plugin struct {
	Type string `json:"type"`
}

type BridgePlugin struct {
	Plugin
	Bridge        string `json:"bridge"`
	IsGateway     bool   `json:"isGateway"`
	IPMasq        bool   `json:"ipMasq"`
	IPMasqBackend string `json:"ipMasqBackend,omitempty"`
	IPAM          IPAM   `json:"ipam"`
	MTU           int    `json:"mtu,omitempty"`
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
	return c.CNIConfigList(FirewallBackendIPTables, false)
}

func (c CNINetworkConfig) ToJSONv6() string {
	return c.CNIConfigList(FirewallBackendIPTables, true)
}

// CNIConfigList is the CNI conflist for one address family. backend is
// FirewallBackendIPTables or FirewallBackendNFTables.
func (c CNINetworkConfig) CNIConfigList(backend string, ipv6 bool) string {
	var subnet *net.IPNet
	var routes []types.Route
	var ipMasq bool
	var err error

	if ipv6 {
		_, subnet, err = net.ParseCIDR(c.IPv6.Subnet)
		if err != nil {
			_, subnet, _ = net.ParseCIDR(DefaultCNINetworkConfig.IPv6.Subnet)
		}
		routes = []types.Route{
			{Dst: *subnet},
			{Dst: *defaultRouteV6},
		}
		ipMasq = c.IPv6.IPMasq
	} else {
		_, subnet, err = net.ParseCIDR(c.IPv4.Subnet)
		if err != nil {
			_, subnet, _ = net.ParseCIDR(DefaultCNINetworkConfig.IPv4.Subnet)
		}
		routes = []types.Route{
			{Dst: *subnet},
			{Dst: *defaultRouteV4},
		}
		ipMasq = true
	}

	bridgePlugin := BridgePlugin{
		Plugin:    Plugin{"bridge"},
		Bridge:    c.BridgeName,
		IsGateway: true,
		IPMasq:    ipMasq,
		MTU:       c.MTU,
		IPAM: IPAM{
			Type: "host-local",
			Ranges: [][]Range{
				{{Subnet: types.IPNet(*subnet)}},
			},
			Routes: routes,
		},
	}
	if backend == FirewallBackendNFTables && ipMasq {
		bridgePlugin.IPMasqBackend = ipMasqBackendNFTables
	}

	plugins := []any{bridgePlugin}
	if backend != FirewallBackendNFTables {
		plugins = append(plugins, defaultFirewallPlugin)
	}

	netConfig := CNINetworkConfiguration{
		Name:       c.NetworkName,
		CNIVersion: "0.4.0",
		Plugins:    plugins,
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

// WithAdditionalHosts defines the additional hosts that will be added to the /etc/hosts file in containers.
func WithAdditionalHosts(additionalHosts []string) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.additionalHosts = additionalHosts
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
		// Tests pass a fake and expect the iptables rules. An empty backend
		// would probe nftables and skip those calls on a host where nft works.
		if n.firewallBackend == "" {
			n.firewallBackend = FirewallBackendIPTables
		}
	}
}

// WithFirewallBackend selects auto, iptables, or nftables. An empty value is auto.
func WithFirewallBackend(backend string) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.firewallBackend = backend
	}
}

// WithNFTables uses an already constructed nftables client and selects the nftables backend
// unless a backend was set explicitly.
func WithNFTables(fw nftables.Firewall) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.nft = fw
		if n.firewallBackend == "" || n.firewallBackend == FirewallBackendAuto {
			n.firewallBackend = FirewallBackendNFTables
		}
	}
}

// WithNFTFactory replaces nftables.New. Tests use it to force a probe result.
func WithNFTFactory(factory func() (nftables.Firewall, error)) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.nftFactory = factory
	}
}

// WithFirewallFallback is called when auto mode cannot initialize nftables and
// the worker continues with iptables.
func WithFirewallFallback(fn func(error)) CNINetworkOpt {
	return func(n *cniNetwork) {
		n.onFirewallFallback = fn
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
	additionalHosts    []string
	binariesDir        string
	restrictedNetworks []string
	allowHostAccess    bool
	firewallBackend    string
	ipt                iptables.Iptables
	nft                nftables.Firewall
	nftFactory         func() (nftables.Firewall, error)
	onFirewallFallback func(error)
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

	if err = n.selectFirewallBackend(); err != nil {
		return nil, err
	}

	if n.client == nil {
		n.client, err = cni.New(cni.WithPluginDir([]string{n.binariesDir}))
		if err != nil {
			return nil, fmt.Errorf("cni init: %w", err)
		}

		cniOpts := []cni.Opt{
			cni.WithConfListBytes([]byte(n.config.CNIConfigList(n.firewallBackend, false))),
			cni.WithLoNetwork,
		}
		if n.config.IPv6.Enabled {
			cniOpts = append(cniOpts, cni.WithConfListBytes([]byte(n.config.CNIConfigList(n.firewallBackend, true))))
		}

		err = n.client.Load(cniOpts...)
		if err != nil {
			return nil, fmt.Errorf("cni configuration loading: %w", err)
		}
	}

	return n, nil
}

func (n *cniNetwork) selectFirewallBackend() error {
	switch n.firewallBackend {
	case "", FirewallBackendAuto:
		factory := n.nftFactory
		if factory == nil {
			factory = nftables.New
		}

		fw, err := factory()
		if err != nil {
			if n.onFirewallFallback != nil {
				n.onFirewallFallback(err)
			}
			n.firewallBackend = FirewallBackendIPTables
			return n.ensureIptables()
		}

		n.nft = fw
		n.firewallBackend = FirewallBackendNFTables
		return nil
	case FirewallBackendNFTables:
		if n.nft != nil {
			return nil
		}

		factory := n.nftFactory
		if factory == nil {
			factory = nftables.New
		}

		fw, err := factory()
		if err != nil {
			return fmt.Errorf("nftables firewall backend: %w", err)
		}
		n.nft = fw
		return nil
	case FirewallBackendIPTables:
		return n.ensureIptables()
	default:
		return fmt.Errorf("unknown firewall backend %q", n.firewallBackend)
	}
}

func (n *cniNetwork) ensureIptables() error {
	if n.ipt != nil {
		return nil
	}

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to initialize iptables: %w", err)
	}
	n.ipt = ipt
	return nil
}

func (n cniNetwork) SetupHostNetwork() error {
	if n.firewallBackend == FirewallBackendNFTables {
		err := n.nft.Setup(n.config.BridgeName, n.restrictedNetworks, n.allowHostAccess)
		if err != nil {
			return fmt.Errorf("setup nftables host network: %w", err)
		}
		return nil
	}

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

func (n cniNetwork) SetupMounts(handle string, hermetic bool) ([]specs.Mount, error) {
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

	// Adding Additional hosts as in guardian original implementation
	// https://github.com/cloudfoundry/guardian/blob/main/kawasaki/dns/hosts_file_compiler.go
	if err := n.addHostsFileEntries(handle); err != nil {
		return nil, fmt.Errorf("adding additional hosts: %w", err)
	}

	etcHostName, err := n.store.Create(
		filepath.Join(handle, "/hostname"),
		[]byte(handle+"\n"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating /etc/hostname: %w", err)
	}

	resolvContents, err := n.generateResolvConfContents(hermetic)
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

func (n cniNetwork) generateResolvConfContents(hermetic bool) ([]byte, error) {
	if hermetic {
		return []byte{}, nil
	}

	contents := ""
	resolvConfEntries := n.nameServers
	var err error

	if len(n.nameServers) == 0 {
		resolvConfEntries, err = ParseHostResolveConf(defaultHostResolvConfPath)
	}

	contents = strings.Join(resolvConfEntries, "\n") + "\n"

	return []byte(contents), err
}

func (n cniNetwork) addHostsFileEntries(handle string) error {
	for _, entry := range n.additionalHosts {
		fields := strings.Fields(entry)
		if len(fields) < 2 {
			return fmt.Errorf("invalid host entry %q: must have IP and hostname separated by a space", entry)
		}
		ip := fields[0]
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid IP in host entry: %q", entry)
		}
		if err := n.store.Append(filepath.Join(handle, "hosts"), []byte(entry+"\n")); err != nil {
			return fmt.Errorf("failed to append host entry %q: %w", entry, err)
		}
	}
	return nil
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
		return errors.Join(ErrGettingContainerIP, err)
	}

	if n.firewallBackend == FirewallBackendNFTables {
		if err := n.nft.DropSource(containerIp); err != nil {
			return fmt.Errorf("error dropping container traffic: %w", err)
		}
		return nil
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
		return errors.Join(ErrGettingContainerIP, err)
	}

	if n.firewallBackend == FirewallBackendNFTables {
		if err := n.nft.DeleteDropSource(containerIp); err != nil {
			return fmt.Errorf("error resuming container traffic: %w", err)
		}
		return nil
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

	if n.firewallBackend == FirewallBackendNFTables {
		for _, ipCfg := range config.IPConfigs {
			if ipCfg == nil || ipCfg.IP == nil {
				continue
			}
			if err := n.nft.AcceptContainer(containerHandle, ipCfg.IP.String()); err != nil {
				return fmt.Errorf("cni nftables setup: %w", err)
			}
		}
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

	if n.firewallBackend == FirewallBackendNFTables {
		if err := n.nft.ForgetContainer(handle); err != nil {
			return fmt.Errorf("cni nftables teardown: %w", err)
		}
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
