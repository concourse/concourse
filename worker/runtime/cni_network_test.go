package runtime_test

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"

	"github.com/containerd/go-cni"

	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/iptables/iptablesfakes"
	"github.com/concourse/concourse/worker/runtime/libcontainerd/libcontainerdfakes"
	"github.com/concourse/concourse/worker/runtime/runtimefakes"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CNINetworkSuite struct {
	suite.Suite
	*require.Assertions

	network  runtime.Network
	cni      *runtimefakes.FakeCNI
	store    *runtimefakes.FakeFileStore
	iptables *iptablesfakes.FakeIptables
}

func (s *CNINetworkSuite) SetupTest() {
	var err error

	s.store = new(runtimefakes.FakeFileStore)
	s.cni = new(runtimefakes.FakeCNI)
	s.iptables = new(iptablesfakes.FakeIptables)

	s.network, err = runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNIFileStore(s.store),
		runtime.WithCNIClient(s.cni),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)
}

func (s *CNINetworkSuite) TestNewCNINetworkWithInvalidIPv4ConfigDoesntFail() {
	_, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNINetworkConfig(runtime.CNINetworkConfig{
			IPv4: runtime.CNIv4NetworkConfig{Subnet: "_____________"},
		}),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)
}

func (s *CNINetworkSuite) TestNewCNINetworkWithInvalidIPv6ConfigDoesntFail() {
	_, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNINetworkConfig(runtime.CNINetworkConfig{
			IPv6: runtime.CNIv6NetworkConfig{Enabled: true, Subnet: "______"},
		}),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)
}

func (s *CNINetworkSuite) TestSetupMountsEmptyHandle() {
	_, err := s.network.SetupMounts("")
	s.EqualError(err, "empty handle")
}

func (s *CNINetworkSuite) TestSetupMountsFailToCreateHosts() {
	s.store.CreateReturnsOnCall(0, "", errors.New("create-hosts-err"))

	_, err := s.network.SetupMounts("handle")
	s.EqualError(errors.Unwrap(err), "create-hosts-err")

	s.Equal(1, s.store.CreateCallCount())
	fname, _ := s.store.CreateArgsForCall(0)

	s.Equal("handle/hosts", fname)
}

func (s *CNINetworkSuite) TestSetupMountsFailToCreateHostname() {
	s.store.CreateReturnsOnCall(1, "", errors.New("create-hostname-err"))

	_, err := s.network.SetupMounts("handle")
	s.EqualError(errors.Unwrap(err), "create-hostname-err")

	s.Equal(2, s.store.CreateCallCount())
	fname, _ := s.store.CreateArgsForCall(1)

	s.Equal("handle/hostname", fname)
}

func (s *CNINetworkSuite) TestSetupMountsFailToCreateResolvConf() {
	s.store.CreateReturnsOnCall(2, "", errors.New("create-resolvconf-err"))

	_, err := s.network.SetupMounts("handle")
	s.EqualError(errors.Unwrap(err), "create-resolvconf-err")

	s.Equal(3, s.store.CreateCallCount())
	fname, _ := s.store.CreateArgsForCall(2)

	s.Equal("handle/resolv.conf", fname)
}

func (s *CNINetworkSuite) TestSetupMountsReturnsMountpoints() {
	s.store.CreateReturnsOnCall(0, "/worker-state/handle/etc/hosts", nil)
	s.store.CreateReturnsOnCall(1, "/worker-state/handle/etc/hostname", nil)
	s.store.CreateReturnsOnCall(2, "/worker-state/handle/etc/resolv.conf", nil)

	mounts, err := s.network.SetupMounts("some-handle")
	s.NoError(err)

	s.Len(mounts, 3)
	s.Equal(mounts, []specs.Mount{
		{
			Destination: "/etc/hosts",
			Type:        "bind",
			Source:      "/worker-state/handle/etc/hosts",
			Options:     []string{"bind", "rw"},
		},
		{
			Destination: "/etc/hostname",
			Type:        "bind",
			Source:      "/worker-state/handle/etc/hostname",
			Options:     []string{"bind", "rw"},
		},
		{
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      "/worker-state/handle/etc/resolv.conf",
			Options:     []string{"bind", "rw"},
		},
	})
}

func (s *CNINetworkSuite) TestSetupMountsCallsStoreWithNameServers() {
	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNIFileStore(s.store),
		runtime.WithNameServers([]string{"6.6.7.7", "1.2.3.4"}),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)

	_, err = network.SetupMounts("some-handle")
	s.NoError(err)

	_, resolvConfContents := s.store.CreateArgsForCall(2)
	s.Equal(resolvConfContents, []byte("nameserver 6.6.7.7\nnameserver 1.2.3.4\n"))
}

func (s *CNINetworkSuite) TestSetupMountsCallsStoreWithoutNameServers() {
	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNIFileStore(s.store),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)

	_, err = network.SetupMounts("some-handle")
	s.NoError(err)

	actualResolvContents, err := runtime.ParseHostResolveConf("/etc/resolv.conf")
	s.NoError(err)

	contents := strings.Join(actualResolvContents, "\n") + "\n"

	_, resolvConfContents := s.store.CreateArgsForCall(2)
	s.Equal(resolvConfContents, []byte(contents))
}

func (s *CNINetworkSuite) TestSetupHostNetwork() {
	testCases := map[string]struct {
		cniNetworkSetup   func() (runtime.Network, error)
		expectedTableName string
		expectedChainName string
		expectedRuleSpec  []string
	}{
		"flushes the CONCOURSE-OPERATOR chain": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
					runtime.WithDefaultsForTesting(),
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "CONCOURSE-OPERATOR",
		},
		"adds rule to CONCOURSE-OPERATOR chain for accepting established connections": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
					runtime.WithDefaultsForTesting(),
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "CONCOURSE-OPERATOR",
			expectedRuleSpec:  []string{"-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		},
		"adds rule to CONCOURSE-OPERATOR chain to reject IP 1.1.1.1": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
					runtime.WithDefaultsForTesting(),
					runtime.WithRestrictedNetworks([]string{"1.1.1.1", "8.8.8.8"}),
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "CONCOURSE-OPERATOR",
			expectedRuleSpec:  []string{"-d", "1.1.1.1", "-j", "REJECT"},
		},
		"adds rule to CONCOURSE-OPERATOR chain to reject IP 8.8.8.8": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
					runtime.WithDefaultsForTesting(),
					runtime.WithRestrictedNetworks([]string{"1.1.1.1", "8.8.8.8"}),
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "CONCOURSE-OPERATOR",
			expectedRuleSpec:  []string{"-d", "8.8.8.8", "-j", "REJECT"},
		},
		"flushes the INPUT chain": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
					runtime.WithDefaultsForTesting(),
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "INPUT",
		},
		"adds rule to INPUT chain to block host access by default": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
					runtime.WithDefaultsForTesting(),
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "INPUT",
			expectedRuleSpec:  []string{"-i", "concourse0", "-j", "REJECT", "--reject-with", "icmp-host-prohibited"},
		},
	}

	for description, testCase := range testCases {
		network, err := testCase.cniNetworkSetup()
		s.NoError(err)
		err = network.SetupHostNetwork()
		s.NoError(err)

		foundExpected := false

		if testCase.expectedRuleSpec == nil {
			// Test cases to check if correct chain is created
			numOfCalls := s.iptables.CreateChainOrFlushIfExistsCallCount()
			for i := 0; i < numOfCalls; i++ {
				tablename, chainName := s.iptables.CreateChainOrFlushIfExistsArgsForCall(i)
				if tablename == testCase.expectedTableName && chainName == testCase.expectedChainName {
					foundExpected = true
					break
				}
			}
		} else {
			// Test cases to check if correct rule is appended
			numOfCalls := s.iptables.AppendRuleCallCount()
			for i := 0; i < numOfCalls; i++ {
				tablename, chainName, rulespec := s.iptables.AppendRuleArgsForCall(i)
				if tablename == testCase.expectedTableName && chainName == testCase.expectedChainName && reflect.DeepEqual(rulespec, testCase.expectedRuleSpec) {
					foundExpected = true
					break
				}
			}

		}

		s.Equal(foundExpected, true, description)
	}
}

func (s *CNINetworkSuite) TestAddNilTask() {
	err := s.network.Add(context.Background(), nil, "container-handle")
	s.EqualError(err, "nil task")
}

func (s *CNINetworkSuite) TestAddSetupErrors() {
	s.cni.SetupReturns(nil, errors.New("setup-err"))
	task := new(libcontainerdfakes.FakeTask)

	err := s.network.Add(context.Background(), task, "container-handle")
	s.EqualError(errors.Unwrap(err), "setup-err")
}

func (s *CNINetworkSuite) TestAddInterfaceNotFound() {
	task := new(libcontainerdfakes.FakeTask)
	task.PidReturns(123)
	task.IDReturns("id")

	result := &cni.Result{
		Interfaces: make(map[string]*cni.Config, 0),
	}
	s.cni.SetupReturns(result, nil)
	err := s.network.Add(context.Background(), task, "container-handle")
	s.EqualError(err, "cni net setup: no eth0 interface found")
}

func (s *CNINetworkSuite) TestAdd() {
	task := new(libcontainerdfakes.FakeTask)
	task.PidReturns(123)
	task.IDReturns("id")

	result := &cni.Result{
		Interfaces: make(map[string]*cni.Config, 0),
	}
	result.Interfaces["eth0"] = &cni.Config{
		IPConfigs: []*cni.IPConfig{
			{
				IP: net.IPv4(10, 8, 0, 1),
			},
		},
	}

	s.cni.SetupReturns(result, nil)

	err := s.network.Add(context.Background(), task, "container-handle")
	s.NoError(err)

	s.Equal(1, s.cni.SetupCallCount())
	_, id, netns, _ := s.cni.SetupArgsForCall(0)
	s.Equal("id", id)
	s.Equal("/proc/123/ns/net", netns)

	s.Equal(s.store.AppendCallCount(), 1)
	path, content := s.store.AppendArgsForCall(0)
	s.Equal(path, "container-handle/hosts")
	s.Equal(content, []byte("10.8.0.1 container-handle\n"))
}

func (s *CNINetworkSuite) TestRemoveNilTask() {
	err := s.network.Remove(context.Background(), nil, "")
	s.EqualError(err, "nil task")
}

func (s *CNINetworkSuite) TestRemoveSetupErrors() {
	s.cni.RemoveReturns(errors.New("remove-err"))
	task := new(libcontainerdfakes.FakeTask)

	err := s.network.Remove(context.Background(), task, "")
	s.EqualError(errors.Unwrap(err), "remove-err")
}

func (s *CNINetworkSuite) TestRemove() {
	task := new(libcontainerdfakes.FakeTask)
	task.PidReturns(123)
	task.IDReturns("id")

	err := s.network.Remove(context.Background(), task, "some-handle")
	s.NoError(err)

	s.Equal(1, s.cni.RemoveCallCount())
	_, id, netns, _ := s.cni.RemoveArgsForCall(0)
	s.Equal("id", id)
	s.Equal("/proc/123/ns/net", netns)

	s.Equal(1, s.store.DeleteCallCount())
	path := s.store.DeleteArgsForCall(0)
	s.Equal("some-handle", path)
}

func (s *CNINetworkSuite) TestDropContainerTraffic() {
	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNIFileStore(s.store),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)

	s.store.ContainerIpLookupReturns("10.8.0.1", nil)

	err = network.DropContainerTraffic("some-handle")
	s.NoError(err)

	s.Equal(s.iptables.InsertRuleCallCount(), 2)
	table, chain, pos, rulespec := s.iptables.InsertRuleArgsForCall(0)
	s.Equal("filter", table)
	s.Equal("INPUT", chain)
	s.Equal(1, pos)
	s.Equal([]string{"-s", "10.8.0.1", "-j", "DROP"}, rulespec)

	table, chain, pos, rulespec = s.iptables.InsertRuleArgsForCall(1)
	s.Equal("filter", table)
	s.Equal("FORWARD", chain)
	s.Equal(1, pos)
	s.Equal([]string{"-s", "10.8.0.1", "-j", "DROP"}, rulespec)
}

func (s *CNINetworkSuite) TestResumeContainerTraffic() {
	network, err := runtime.NewCNINetwork(
		runtime.WithDefaultsForTesting(),
		runtime.WithCNIFileStore(s.store),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)

	s.store.ContainerIpLookupReturns("10.8.0.1", nil)

	err = network.ResumeContainerTraffic("some-handle")
	s.NoError(err)

	s.Equal(s.iptables.DeleteRuleCallCount(), 2)
	table, chain, rulespec := s.iptables.DeleteRuleArgsForCall(0)
	s.Equal("filter", table)
	s.Equal("INPUT", chain)
	s.Equal([]string{"-s", "10.8.0.1", "-j", "DROP"}, rulespec)

	table, chain, rulespec = s.iptables.DeleteRuleArgsForCall(1)
	s.Equal("filter", table)
	s.Equal("FORWARD", chain)
	s.Equal([]string{"-s", "10.8.0.1", "-j", "DROP"}, rulespec)
}
