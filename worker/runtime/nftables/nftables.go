//go:build linux

package nftables

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"sigs.k8s.io/knftables"
)

const (
	// TableName is the inet table that holds the worker's container firewall.
	TableName = "concourse"

	forwardChain  = "forward"
	inputChain    = "input"
	operatorChain = "operator"

	allowed4 = "allowed4"
	allowed6 = "allowed6"
	dropped4 = "dropped4"
	dropped6 = "dropped6"

	// filterHookPriority runs before the iptables filter hook so a reject or
	// drop here wins, while an accept still lets later hooks see the packet.
	filterHookPriority = knftables.BaseChainPriority("filter - 10")
)

// Firewall is the host policy for container traffic when the worker is using nftables.
// The CNI firewall plugin is not in the network config on this path, so Setup installs
// the forward and input chains that plugin would have jumped to.
type Firewall interface {
	Setup(bridge string, restricted []string, allowHostAccess bool) error
	AcceptContainer(handle, ip string) error
	ForgetContainer(handle string) error
	DropSource(ip string) error
	DeleteDropSource(ip string) error
}

type client struct {
	nft knftables.Interface

	mu       sync.Mutex
	byHandle map[string][]string
}

// New checks that nftables can create the worker table. A failure means the
// caller should stay on iptables.
func New() (Firewall, error) {
	nft, err := knftables.New(knftables.InetFamily, TableName)
	if err != nil {
		return nil, err
	}

	c := newClient(nft)
	if err := c.ensureTable(); err != nil {
		return nil, err
	}

	return c, nil
}

func newClient(nft knftables.Interface) *client {
	return &client{
		nft:      nft,
		byHandle: make(map[string][]string),
	}
}

func (c *client) Setup(bridge string, restricted []string, allowHostAccess bool) error {
	iface, err := quoteIface(bridge)
	if err != nil {
		return err
	}

	rejects := make([]string, 0, len(restricted))
	for _, network := range restricted {
		rule, err := rejectRule(network)
		if err != nil {
			return err
		}
		rejects = append(rejects, rule)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	tx := c.nft.NewTransaction()
	addTable(tx)
	addSets(tx)
	tx.Flush(&knftables.Set{Name: allowed4})
	tx.Flush(&knftables.Set{Name: allowed6})
	tx.Flush(&knftables.Set{Name: dropped4})
	tx.Flush(&knftables.Set{Name: dropped6})

	tx.Add(&knftables.Chain{Name: operatorChain})
	tx.Flush(&knftables.Chain{Name: operatorChain})
	tx.Add(&knftables.Rule{
		Chain: operatorChain,
		Rule:  "ct state established,related accept",
	})
	for _, rule := range rejects {
		tx.Add(&knftables.Rule{Chain: operatorChain, Rule: rule})
	}

	tx.Add(baseChain(forwardChain, knftables.ForwardHook))
	tx.Flush(&knftables.Chain{Name: forwardChain})
	for _, rule := range forwardRules(iface) {
		tx.Add(&knftables.Rule{Chain: forwardChain, Rule: rule})
	}

	tx.Add(baseChain(inputChain, knftables.InputHook))
	tx.Flush(&knftables.Chain{Name: inputChain})
	for _, rule := range inputRules(iface, allowHostAccess) {
		tx.Add(&knftables.Rule{Chain: inputChain, Rule: rule})
	}

	if err := c.nft.Run(context.Background(), tx); err != nil {
		return fmt.Errorf("nftables setup: %w", err)
	}

	c.byHandle = make(map[string][]string)
	return nil
}

func (c *client) AcceptContainer(handle, ip string) error {
	canonical, v4, err := canonicalIP(ip)
	if err != nil {
		return err
	}

	setName := allowed6
	if v4 {
		setName = allowed4
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.addElement(setName, canonical); err != nil {
		return fmt.Errorf("allow container %s: %w", handle, err)
	}

	for _, existing := range c.byHandle[handle] {
		if existing == canonical {
			return nil
		}
	}
	c.byHandle[handle] = append(c.byHandle[handle], canonical)
	return nil
}

func (c *client) ForgetContainer(handle string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ips := c.byHandle[handle]
	delete(c.byHandle, handle)

	var first error
	for _, ip := range ips {
		setName := allowed6
		if net.ParseIP(ip).To4() != nil {
			setName = allowed4
		}
		if err := c.deleteElement(setName, ip); err != nil && first == nil {
			first = fmt.Errorf("forget container %s: %w", handle, err)
		}
	}
	return first
}

func (c *client) DropSource(ip string) error {
	canonical, v4, err := canonicalIP(ip)
	if err != nil {
		return err
	}

	setName := dropped6
	if v4 {
		setName = dropped4
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.addElement(setName, canonical); err != nil {
		return fmt.Errorf("drop source %s: %w", canonical, err)
	}
	return nil
}

func (c *client) DeleteDropSource(ip string) error {
	canonical, v4, err := canonicalIP(ip)
	if err != nil {
		return err
	}

	setName := dropped6
	if v4 {
		setName = dropped4
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.deleteElement(setName, canonical); err != nil {
		return fmt.Errorf("resume source %s: %w", canonical, err)
	}
	return nil
}

func (c *client) ensureTable() error {
	tx := c.nft.NewTransaction()
	addTable(tx)
	return c.nft.Run(context.Background(), tx)
}

func (c *client) addElement(setName, ip string) error {
	tx := c.nft.NewTransaction()
	tx.Add(&knftables.Element{Set: setName, Key: []string{ip}})
	return c.nft.Run(context.Background(), tx)
}

func (c *client) deleteElement(setName, ip string) error {
	tx := c.nft.NewTransaction()
	tx.Delete(&knftables.Element{Set: setName, Key: []string{ip}})
	err := c.nft.Run(context.Background(), tx)
	if knftables.IsNotFound(err) {
		return nil
	}
	return err
}

func addTable(tx *knftables.Transaction) {
	tx.Add(&knftables.Table{
		Comment: knftables.PtrTo("concourse container firewall"),
	})
}

func addSets(tx *knftables.Transaction) {
	for _, set := range []*knftables.Set{
		{Name: allowed4, Type: "ipv4_addr"},
		{Name: allowed6, Type: "ipv6_addr"},
		{Name: dropped4, Type: "ipv4_addr"},
		{Name: dropped6, Type: "ipv6_addr"},
	} {
		tx.Add(set)
	}
}

func baseChain(name string, hook knftables.BaseChainHook) *knftables.Chain {
	return &knftables.Chain{
		Name:     name,
		Type:     knftables.PtrTo(knftables.FilterType),
		Hook:     knftables.PtrTo(hook),
		Priority: knftables.PtrTo(filterHookPriority),
		Policy:   knftables.PtrTo(knftables.AcceptPolicy),
	}
}

func forwardRules(iface string) []string {
	return []string{
		"ip saddr @" + dropped4 + " drop",
		"ip6 saddr @" + dropped6 + " drop",
		"jump " + operatorChain,
		"ip saddr @" + allowed4 + " accept",
		"ip6 saddr @" + allowed6 + " accept",
		"ip daddr @" + allowed4 + " ct state established,related accept",
		"ip6 daddr @" + allowed6 + " ct state established,related accept",
		"iifname " + iface + " drop",
	}
}

func inputRules(iface string, allowHostAccess bool) []string {
	rules := []string{
		"ip saddr @" + dropped4 + " drop",
		"ip6 saddr @" + dropped6 + " drop",
	}
	if !allowHostAccess {
		rules = append(rules, "iifname "+iface+" reject")
	}
	return rules
}

func rejectRule(network string) (string, error) {
	if ip := net.ParseIP(network); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return "ip daddr " + v4.String() + " reject", nil
		}
		return "ip6 daddr " + ip.String() + " reject", nil
	}

	_, cidr, err := net.ParseCIDR(network)
	if err != nil {
		return "", fmt.Errorf("restricted network %q is not an IP or CIDR", network)
	}
	if cidr.IP.To4() != nil {
		return "ip daddr " + cidr.String() + " reject", nil
	}
	return "ip6 daddr " + cidr.String() + " reject", nil
}

func canonicalIP(ip string) (string, bool, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", false, fmt.Errorf("invalid IP %q", ip)
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String(), true, nil
	}
	return parsed.String(), false, nil
}

func quoteIface(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("bridge name is empty")
	}
	if strings.ContainsAny(name, "\\\"\n\t ") {
		return "", fmt.Errorf("bridge name %q contains unsupported characters", name)
	}
	return `"` + name + `"`, nil
}
