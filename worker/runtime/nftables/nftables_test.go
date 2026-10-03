//go:build linux

package nftables

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/knftables"
)

func testClient(t *testing.T) (*client, *knftables.Fake) {
	t.Helper()
	fake := knftables.NewFake(knftables.InetFamily, TableName)
	c := newClient(fake)
	require.NoError(t, c.ensureTable())
	return c, fake
}

func ruleText(t *testing.T, fake *knftables.Fake, chain string) []string {
	t.Helper()
	fake.RLock()
	defer fake.RUnlock()

	got := make([]string, 0)
	for _, rule := range fake.Table.Chains[chain].Rules {
		got = append(got, rule.Rule)
	}
	return got
}

func hasElement(t *testing.T, fake *knftables.Fake, setName, ip string) bool {
	t.Helper()
	fake.RLock()
	defer fake.RUnlock()
	return fake.Table.Sets[setName].FindElement(ip) != nil
}

func TestSetupInstallsForwardAndInputPolicy(t *testing.T) {
	c, fake := testClient(t)

	err := c.Setup("concourse0", []string{"1.1.1.1", "2001:db8::/64"}, false)
	require.NoError(t, err)

	require.Equal(t, []string{
		"ip saddr @dropped4 drop",
		"ip6 saddr @dropped6 drop",
		"jump operator",
		"ip saddr @allowed4 accept",
		"ip6 saddr @allowed6 accept",
		"ip daddr @allowed4 ct state established,related accept",
		"ip6 daddr @allowed6 ct state established,related accept",
		`iifname "concourse0" drop`,
	}, ruleText(t, fake, forwardChain))

	require.Equal(t, []string{
		"ct state established,related accept",
		"ip daddr 1.1.1.1 reject",
		"ip6 daddr 2001:db8::/64 reject",
	}, ruleText(t, fake, operatorChain))

	require.Equal(t, []string{
		"ip saddr @dropped4 drop",
		"ip6 saddr @dropped6 drop",
		`iifname "concourse0" reject`,
	}, ruleText(t, fake, inputChain))
}

func TestSetupAllowHostAccessSkipsBridgeReject(t *testing.T) {
	c, fake := testClient(t)

	err := c.Setup("concourse0", nil, true)
	require.NoError(t, err)

	require.Equal(t, []string{
		"ip saddr @dropped4 drop",
		"ip6 saddr @dropped6 drop",
	}, ruleText(t, fake, inputChain))
}

func TestSetupRejectsBadRestrictedNetwork(t *testing.T) {
	c, _ := testClient(t)

	err := c.Setup("concourse0", []string{"not-a-network"}, false)
	require.Error(t, err)
}

func TestAcceptAndForgetContainer(t *testing.T) {
	c, fake := testClient(t)
	require.NoError(t, c.Setup("concourse0", nil, false))

	require.NoError(t, c.AcceptContainer("task", "10.80.0.8"))
	require.NoError(t, c.AcceptContainer("task", "fd9c:31a6:c759::8"))
	require.True(t, hasElement(t, fake, allowed4, "10.80.0.8"))
	require.True(t, hasElement(t, fake, allowed6, "fd9c:31a6:c759::8"))

	require.NoError(t, c.ForgetContainer("task"))
	require.False(t, hasElement(t, fake, allowed4, "10.80.0.8"))
	require.False(t, hasElement(t, fake, allowed6, "fd9c:31a6:c759::8"))
	require.NoError(t, c.ForgetContainer("task"))
}

func TestDropAndResumeSource(t *testing.T) {
	c, fake := testClient(t)
	require.NoError(t, c.Setup("concourse0", nil, false))

	require.NoError(t, c.DropSource("10.80.0.9"))
	require.True(t, hasElement(t, fake, dropped4, "10.80.0.9"))

	require.NoError(t, c.DeleteDropSource("10.80.0.9"))
	require.False(t, hasElement(t, fake, dropped4, "10.80.0.9"))
	require.NoError(t, c.DeleteDropSource("10.80.0.9"))
}

func TestSetupClearsStaleContainerAllows(t *testing.T) {
	c, fake := testClient(t)
	require.NoError(t, c.Setup("concourse0", nil, false))
	require.NoError(t, c.AcceptContainer("task", "10.80.0.8"))

	require.NoError(t, c.Setup("concourse0", nil, false))
	require.False(t, hasElement(t, fake, allowed4, "10.80.0.8"))
	require.NoError(t, c.ForgetContainer("task"))
}
