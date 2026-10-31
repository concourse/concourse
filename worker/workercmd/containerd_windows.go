//go:build windows

package workercmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager/v3"
	concourseCmd "github.com/concourse/concourse/v8/cmd"
	"github.com/concourse/concourse/v8/worker/runtime"
	"github.com/concourse/concourse/v8/worker/runtime/libcontainerd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

const containerdNamespace = "concourse"

// containerdAddr is the named pipe on which the containerd daemon spawned by
// the worker listens.
const containerdAddr = `\\.\pipe\containerd-concourse`

// WriteDefaultContainerdConfig writes a default containerd configuration file
// to a destination.
func WriteDefaultContainerdConfig(dest string) error {
	// disable the CRI plugin: we're not supposed to be targetted by a
	// kubelet, so there's no need to bring up kubernete's container runtime
	// interface plugin.
	const config = `
version = 4

disabled_plugins = [
	"io.containerd.grpc.v1.cri",
	"io.containerd.cri.v1.images",
	"io.containerd.cri.v1.runtime",
]

[debug]
  format = "json"
`
	err := os.WriteFile(dest, []byte(config), 0755)
	if err != nil {
		return fmt.Errorf("write file %s: %w", dest, err)
	}

	return nil
}

// containerdGardenServerRunner launches a Garden server configured to interact
// with containerd via the containerdAddr named pipe.
func (cmd *WorkerCommand) containerdGardenServerRunner(logger lager.Logger, containerdAddr string) (ifrit.Runner, error) {
	const graceTime = 0

	if cmd.Containerd.InitBin == "" {
		initBin := concourseCmd.DiscoverAsset("bin/init.exe")
		if initBin == "" {
			return nil, fmt.Errorf("could not find init binary. Try setting the --containerd-init-bin flag")
		}
		cmd.Containerd.InitBin = initBin
	}

	network := runtime.NewHCNNetwork(
		runtime.WithHCNNetworkName(cmd.Containerd.Network.Name),
		runtime.WithHCNNetworkPool(cmd.Containerd.Network.Pool, cmd.Containerd.Network.Gateway),
	)

	backendOpts := []runtime.GardenBackendOpt{
		runtime.WithNetwork(network),
		runtime.WithRequestTimeout(cmd.Containerd.RequestTimeout),
		runtime.WithMaxContainers(cmd.Containerd.MaxContainers),
		runtime.WithInitBinPath(cmd.Containerd.InitBin),
	}

	gardenBackend, err := runtime.NewGardenBackend(
		libcontainerd.New(containerdAddr, containerdNamespace, cmd.Containerd.RequestTimeout),
		backendOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("containerd backend init: %w", err)
	}

	return newGardenServerRunner(
		"tcp",
		cmd.bindAddr(),
		graceTime,
		&gardenBackend,
		logger,
	), nil
}

// containerdRunner spawns a containerd and a Garden server process for use as the container
// runtime of Concourse.
func (cmd *WorkerCommand) containerdRunner(logger lager.Logger) (ifrit.Runner, error) {
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
		"--address="+containerdAddr,
		"--root="+root,
		"--config="+config,
		"--log-level="+cmd.Containerd.LogLevel,
	)

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	gardenServerRunner, err := cmd.containerdGardenServerRunner(logger, containerdAddr)
	if err != nil {
		return nil, fmt.Errorf("containerd garden server runner: %w", err)
	}

	members := grouper.Members{
		{
			Name: "containerd",
			Runner: CmdRunner{
				Cmd: command,
				Ready: func() bool {
					client := libcontainerd.New(containerdAddr, containerdNamespace, cmd.Containerd.RequestTimeout)
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
	}

	// Using the Ordered strategy to ensure containerd is up before the garden server is started
	return grouper.NewOrdered(os.Interrupt, members), nil
}
