//go:build linux

package workercmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/localip"
	concourseCmd "github.com/concourse/concourse/cmd"
	"github.com/concourse/concourse/worker/network"
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

const containerdNamespace = "concourse"

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
version = 3

oom_score = -999
disabled_plugins = ["io.containerd.grpc.v1.cri", "io.containerd.snapshotter.v1.aufs", "io.containerd.snapshotter.v1.btrfs", "io.containerd.snapshotter.v1.zfs"]
`
	err := os.WriteFile(dest, []byte(config), 0755)
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
	const graceTime = 0

	cniNetwork, err := cmd.buildUpNetworkOpts(logger, dnsServers)
	if err != nil {
		return nil, fmt.Errorf("failed to create CNI network opts: %w", err)
	}

	backendOpts, err := cmd.buildUpBackendOpts(logger, cniNetwork)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd backend opts: %w", err)
	}

	gardenBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(containerdAddr, containerdNamespace, cmd.Containerd.RequestTimeout),
		backendOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("containerd containerd init: %w", err)
	}

	return newGardenServerRunner(
		"tcp",
		cmd.bindAddr(),
		graceTime,
		&gardenBackend,
		logger,
	), nil
}

func (cmd *WorkerCommand) buildUpNetworkOpts(logger lager.Logger, dnsServers []string) (runtime.Network, error) {
	logger.Debug("create-cni-network-opts")
	if cmd.Containerd.CNIPluginsDir == "" {
		pluginsDir := concourseCmd.DiscoverAsset("bin")
		if pluginsDir == "" {
			return nil, fmt.Errorf("could not find CNI Plugins dir. Try setting the --containerd-cni-plugins-dir flag")
		}
		cmd.Containerd.CNIPluginsDir = pluginsDir
	}

	networkOpts := []runtime.CNINetworkOpt{
		runtime.WithCNIBinariesDir(cmd.Containerd.CNIPluginsDir),
		runtime.WithCNIFileStore(runtime.FileStoreWithWorkDir(cmd.WorkDir.Path())),
	}

	if len(dnsServers) > 0 {
		networkOpts = append(networkOpts, runtime.WithNameServers(dnsServers))
	}

	if len(cmd.Containerd.Network.AdditionalHosts) > 0 {
		networkOpts = append(networkOpts, runtime.WithAdditionalHosts(cmd.Containerd.Network.AdditionalHosts))
	}

	if len(cmd.Containerd.Network.RestrictedNetworks) > 0 {
		networkOpts = append(networkOpts, runtime.WithRestrictedNetworks(cmd.Containerd.Network.RestrictedNetworks))
	}

	// DNS proxy won't work without allowing access to host network
	if cmd.Containerd.Network.AllowHostAccess || cmd.Containerd.Network.DNS.Enable {
		networkOpts = append(networkOpts, runtime.WithAllowHostAccess())
	}

	networkConfig := runtime.DefaultCNINetworkConfig
	if cmd.Containerd.Network.Pool != "" {
		networkConfig.IPv4.Subnet = cmd.Containerd.Network.Pool
	}

	networkConfig.IPv6 = runtime.CNIv6NetworkConfig{
		Enabled: cmd.Containerd.Network.IPv6.Enable,
		Subnet:  cmd.Containerd.Network.IPv6.Pool,
		IPMasq:  !cmd.Containerd.Network.IPv6.DisableIPMasq,
	}

	var err error
	networkConfig.MTU, err = cmd.Containerd.mtu()
	if err != nil {
		return nil, fmt.Errorf("container MTU: %w", err)
	}
	networkOpts = append(networkOpts, runtime.WithCNINetworkConfig(networkConfig))

	return runtime.NewCNINetwork(networkOpts...)
}

func (cmd *WorkerCommand) buildUpBackendOpts(logger lager.Logger, cniNetwork runtime.Network) ([]runtime.GardenBackendOpt, error) {
	logger.Debug("create-containerd-backendOpts")

	if cmd.Containerd.InitBin == "" {
		initBin := concourseCmd.DiscoverAsset("bin/init")
		if initBin == "" {
			return nil, fmt.Errorf("could not find init binary. Try setting the --containerd-init-bin flag")
		}
		cmd.Containerd.InitBin = initBin
	}

	return []runtime.GardenBackendOpt{
		runtime.WithNetwork(cniNetwork),
		runtime.WithRequestTimeout(cmd.Containerd.RequestTimeout),
		runtime.WithMaxContainers(cmd.Containerd.MaxContainers),
		runtime.WithInitBinPath(cmd.Containerd.InitBin),
		runtime.WithSeccompProfilePath(cmd.Containerd.SeccompProfilePath),
		runtime.WithOciHooksDir(cmd.Containerd.OCIHooksDir),
		runtime.WithPrivilegedMode(cmd.Containerd.PrivilegedMode),
	}, nil
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
		"--log-level="+cmd.Containerd.LogLevel,
	)

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Setpgid:   true,
	}

	members := grouper.Members{}

	dnsServers := cmd.Containerd.Network.DNSServers
	if cmd.Containerd.Network.DNS.Enable {
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
			Name: "containerd",
			Runner: CmdRunner{
				Cmd: command,
				Ready: func() bool {
					client := libcontainerd.New(sock, containerdNamespace, cmd.Containerd.RequestTimeout)
					err := client.Init()
					if err != nil {
						logger.Info("failed-to-connect-to-containerd", lager.Data{"error": err.Error()})
					}
					return err == nil
				},
				Timeout: 60 * time.Second,
			},
		},
		{
			Name:   "containerd-garden-backend",
			Runner: gardenServerRunner,
		},
	}, members...)

	// Using the Ordered strategy to ensure containerd is up before the garden server is started
	return grouper.NewOrdered(os.Interrupt, members), nil
}

func (cmd ContainerdRuntime) externalIP() (net.IP, error) {
	if cmd.Network.ExternalIP.IP != nil {
		return cmd.Network.ExternalIP.IP, nil
	}

	localIP, err := localip.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("Couldn't determine local IP to use for --external-ip parameter. You can use the --external-ip flag to pass an external IP explicitly.")
	}

	return net.ParseIP(localIP), nil
}

func (cmd ContainerdRuntime) mtu() (int, error) {
	if cmd.Network.MTU != 0 {
		return cmd.Network.MTU, nil
	}
	externalIP, err := cmd.externalIP()
	if err != nil {
		return 0, err
	}

	return network.MTU(externalIP.String())
}
