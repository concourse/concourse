// +build linux

package workercmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/localip"
	concourseCmd "github.com/concourse/concourse/cmd"
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/restart"
)

func containerdGardenServerRunner(
	logger lager.Logger,
	bindAddr,
	containerdAddr string,
	requestTimeout time.Duration,
	dnsServers []string,
	networkPool string,
) (ifrit.Runner, error) {
	const (
		graceTime = 0
		namespace = "concourse"
	)

	backendOpts := []runtime.GardenBackendOpt{}
	networkOpts := []runtime.CNINetworkOpt{}

	if len(dnsServers) > 0 {
		networkOpts = append(networkOpts, runtime.WithNameServers(dnsServers))
	}

	if networkPool != "" {
		networkOpts = append(networkOpts, runtime.WithCNINetworkConfig(
			runtime.CNINetworkConfig{
				BridgeName:  "concourse0",
				NetworkName: "concourse",
				Subnet:      networkPool,
			}))
	}

	cniNetwork, err := runtime.NewCNINetwork(networkOpts...)
	if err != nil {
		return nil, fmt.Errorf("new cni network: %w", err)
	}

	backendOpts = append(backendOpts, runtime.WithNetwork(cniNetwork))

	gardenBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(containerdAddr, namespace, requestTimeout),
		backendOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("containerd containerd init: %w", err)
	}

	server := server.New("tcp", bindAddr,
		graceTime,
		&gardenBackend,
		logger,
	)

	runner := gardenServerRunner{logger, server}

	return restart.Restarter{
		Runner: runner,
		Load: func(prevRunner ifrit.Runner, prevErr error) ifrit.Runner {
			return runner
		},
	}, nil
}

// writeDefaultContainerdConfig writes a default containerd configuration file
// to a destination.
//
func writeDefaultContainerdConfig(dest string) error {
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
	const config = `disabled_plugins = ["cri", "aufs", "btrfs", "zfs"]`

	err := ioutil.WriteFile(dest, []byte(config), 0755)
	if err != nil {
		return fmt.Errorf("write file %s: %w", dest, err)
	}

	return nil
}

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

	if cmd.Garden.Config.Path() != "" {
		config = cmd.Garden.Config.Path()
	} else {
		err := writeDefaultContainerdConfig(config)
		if err != nil {
			return nil, fmt.Errorf("write default containerd config: %w", err)
		}
	}

	if cmd.Garden.Bin != "" {
		bin = cmd.Garden.Bin
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

	dnsServers := cmd.Garden.DNSServers
	if cmd.Garden.DNS.Enable {
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

	gardenServerRunner, err := containerdGardenServerRunner(
		logger,
		cmd.bindAddr(),
		sock,
		cmd.Garden.RequestTimeout,
		dnsServers,
		cmd.ContainerNetworkPool,
	)
	if err != nil {
		return nil, fmt.Errorf("containerd garden server runner: %w", err)
	}

	members = append(members, grouper.Members{
		{
			Name:   "containerd",
			Runner: CmdRunner{command},
		},
		{
			Name:   "containerd-garden-backend",
			Runner: gardenServerRunner,
		},
	}...)

	return grouper.NewParallel(os.Interrupt, members), nil
}
