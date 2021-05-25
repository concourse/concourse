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
		runtime.WithCNIFileStore(s.store),
		runtime.WithCNIClient(s.cni),
		runtime.WithIptables(s.iptables),
	)
	s.NoError(err)
}

func (s *CNINetworkSuite) TestNewCNINetworkWithInvalidConfigDoesntFail() {
	// CNI defers the actual interpretation of the network configuration to
	// the plugins.
	//
	_, err := runtime.NewCNINetwork(
		runtime.WithCNINetworkConfig(runtime.CNINetworkConfig{
			Subnet: "_____________",
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
	s.store.CreateReturnsOnCall(0, "/tmp/handle/etc/hosts", nil)
	s.store.CreateReturnsOnCall(1, "/tmp/handle/etc/hostname", nil)
	s.store.CreateReturnsOnCall(2, "/tmp/handle/etc/resolv.conf", nil)

	mounts, err := s.network.SetupMounts("some-handle")
	s.NoError(err)

	s.Len(mounts, 3)
	s.Equal(mounts, []specs.Mount{
		{
			Destination: "/etc/hosts",
			Type:        "bind",
			Source:      "/tmp/handle/etc/hosts",
			Options:     []string{"bind", "rw"},
		},
		{
			Destination: "/etc/hostname",
			Type:        "bind",
			Source:      "/tmp/handle/etc/hostname",
			Options:     []string{"bind", "rw"},
		},
		{
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      "/tmp/handle/etc/resolv.conf",
			Options:     []string{"bind", "rw"},
		},
	})
}

func (s *CNINetworkSuite) TestSetupMountsCallsStoreWithNameServers() {
	network, err := runtime.NewCNINetwork(
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
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "CONCOURSE-OPERATOR",
		},
		"adds rule to CONCOURSE-OPERATOR chain for accepting established connections": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
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
					runtime.WithIptables(s.iptables),
				)
			},
			expectedTableName: "filter",
			expectedChainName: "INPUT",
		},
		"adds rule to INPUT chain to block host access by default": {
			cniNetworkSetup: func() (runtime.Network, error) {
				return runtime.NewCNINetwork(
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
}

func (s *CNINetworkSuite) TestRemoveNilTask() {
	err := s.network.Remove(context.Background(), nil)
	s.EqualError(err, "nil task")
}

func (s *CNINetworkSuite) TestRemoveSetupErrors() {
	s.cni.RemoveReturns(errors.New("remove-err"))
	task := new(libcontainerdfakes.FakeTask)

	err := s.network.Remove(context.Background(), task)
	s.EqualError(errors.Unwrap(err), "remove-err")
}

func (s *CNINetworkSuite) TestRemove() {
	task := new(libcontainerdfakes.FakeTask)
	task.PidReturns(123)
	task.IDReturns("id")

	err := s.network.Remove(context.Background(), task)
	s.NoError(err)

	s.Equal(1, s.cni.RemoveCallCount())
	_, id, netns, _ := s.cni.RemoveArgsForCall(0)
	s.Equal("id", id)
	s.Equal("/proc/123/ns/net", netns)
}
