// +build linux

package workercmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/localip"
	concourseCmd "github.com/concourse/concourse/cmd"
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

// WriteDefaultContainerdConfig writes a default containerd configuration file
// to a destination.
func WriteDefaultContainerdConfig(dest string) error {
	// disable plugins we don't use:
	//
	// - CRI: we're not supposed to be targetted by a kubelet, so there's no
	//        need to bring up kubernete's container runtime interface plugin.
	//
	// - AUFS/BTRFS/ZFS: since linux 3.18, `overlayfs` is in upstream, which
	//                   most distros should include, so by keeping a focus
	//                   on a single snapshotter implementation we can better
	//                   reason about potential problems down the road.
	//
	const config = `
oom_score = -999
disabled_plugins = ["cri", "aufs", "btrfs", "zfs"]
`
	err := ioutil.WriteFile(dest, []byte(config), 0755)
	if err != nil {
		return fmt.Errorf("write file %s: %w", dest, err)
	}

	return nil
}

// containerdGardenServerRunner launches a Garden server configured to interact
// with containerd via the containerdAddr socket.
func (cmd *WorkerCommand) containerdGardenServerRunner(
	logger lager.Logger,
	containerdAddr string,
	dnsServers []string,
) (ifrit.Runner, error) {
	const (
		graceTime = 0
		namespace = "concourse"
	)

	backendOpts := []runtime.GardenBackendOpt{}
	networkOpts := []runtime.CNINetworkOpt{runtime.WithCNIBinariesDir(cmd.Containerd.CNIPluginsDir)}

	if len(dnsServers) > 0 {
		networkOpts = append(networkOpts, runtime.WithNameServers(dnsServers))
	}

	if len(cmd.Containerd.RestrictedNetworks) > 0 {
		networkOpts = append(networkOpts, runtime.WithRestrictedNetworks(cmd.Containerd.RestrictedNetworks))
	}

	if cmd.Containerd.NetworkPool != "" {
		networkOpts = append(networkOpts, runtime.WithCNINetworkConfig(
			runtime.CNINetworkConfig{
				BridgeName:  "concourse0",
				NetworkName: "concourse",
				Subnet:      cmd.Containerd.NetworkPool,
			}))
	}

	cniNetwork, err := runtime.NewCNINetwork(networkOpts...)
	if err != nil {
		return nil, fmt.Errorf("new cni network: %w", err)
	}

	backendOpts = append(backendOpts,
		runtime.WithNetwork(cniNetwork),
		runtime.WithRequestTimeout(cmd.Containerd.RequestTimeout),
		runtime.WithMaxContainers(cmd.Containerd.MaxContainers),
		runtime.WithInitBinPath(cmd.Containerd.InitBin),
	)

	gardenBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(containerdAddr, namespace, cmd.Containerd.RequestTimeout),
		backendOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("containerd containerd init: %w", err)
	}

	server := server.New("tcp", cmd.bindAddr(),
		graceTime,
		&gardenBackend,
		logger,
	)

	return gardenServerRunner{logger, server}, nil
}

// containerdRunner spawns a containerd and a Garden server process for use as the container
// runtime of Concourse.
func (cmd *WorkerCommand) containerdRunner(logger lager.Logger) (ifrit.Runner, error) {
	const sock = "/run/containerd/containerd.sock"

	var (
		config = filepath.Join(cmd.WorkDir.Path(), "containerd.toml")
		root   = filepath.Join(cmd.WorkDir.Path(), "containerd")
		bin    = "containerd"
	)

	err := os.MkdirAll(root, 0755)
	if err != nil {
		return nil, err
	}

	if cmd.Containerd.Config.Path() != "" {
		config = cmd.Containerd.Config.Path()
	} else {
		err := WriteDefaultContainerdConfig(config)
		if err != nil {
			return nil, fmt.Errorf("write default containerd config: %w", err)
		}
	}

	if cmd.Containerd.Bin != "" {
		bin = cmd.Containerd.Bin
	}

	command := exec.Command(bin,
		"--address="+sock,
		"--root="+root,
		"--config="+config,
	)

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	members := grouper.Members{}

	dnsServers := cmd.Containerd.DNSServers
	if cmd.Containerd.DNS.Enable {
		dnsProxyRunner, err := cmd.dnsProxyRunner(logger.Session("dns-proxy"))
		if err != nil {
			return nil, err
		}

		lip, err := localip.LocalIP()
		if err != nil {
			return nil, err
		}

		dnsServers = append(dnsServers, lip)

		members = append(members, grouper.Member{
			Name: "dns-proxy",
			Runner: concourseCmd.NewLoggingRunner(
				logger.Session("dns-proxy-runner"),
				dnsProxyRunner,
			),
		})
	}

	gardenServerRunner, err := cmd.containerdGardenServerRunner(
		logger,
		sock,
		dnsServers,
	)
	if err != nil {
		return nil, fmt.Errorf("containerd garden server runner: %w", err)
	}

	members = append(grouper.Members{
		{
			Name:   "containerd",
			Runner: CmdRunner{command},
		},
		{
			Name:   "containerd-garden-backend",
			Runner: gardenServerRunner,
		},
	}, members...)

	// Using the Ordered strategy to ensure containerd is up before the garden server is started
	return grouper.NewOrdered(os.Interrupt, members), nil
}
