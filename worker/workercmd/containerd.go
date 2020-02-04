// +build linux

package workercmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/restart"
)

func containerdGardenServerRunner(logger lager.Logger, bindAddr, containerdAddr string) ifrit.Runner {
	const (
		graceTime = 0
		namespace = "concourse"
	)

	backend := backend.New(libcontainerd.New(containerdAddr), namespace)

	server := server.New("tcp", bindAddr,
		graceTime,
		&backend,
		logger,
	)

	runner := gardenServerRunner{logger, server}

	return restart.Restarter{
		Runner: runner,
		Load: func(prevRunner ifrit.Runner, prevErr error) ifrit.Runner {
			return runner
		},
	}
}

func (cmd *WorkerCommand) containerdRunner(logger lager.Logger) ifrit.Runner {
	var (
		sock = filepath.Join(cmd.WorkDir.Path(), "containerd.sock")
		root = filepath.Join(cmd.WorkDir.Path(), "containerd")
		bin  = "containerd"
	)

	args := []string{
		"--address=" + sock,
		"--root=" + root,
	}

	if cmd.Garden.Config.Path() != "" {
		args = append(args, "--config", cmd.Garden.Config.Path())
	}

	if cmd.Garden.Bin != "" {
		bin = cmd.Garden.Bin
	}

	command := exec.Command(bin, args...)

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	return grouper.NewParallel(os.Interrupt, grouper.Members{
		{
			Name:   "containerd",
			Runner: CmdRunner{command},
		},
		{
			Name: "containerd-backend",
			Runner: containerdGardenServerRunner(
				logger, cmd.bindAddr(), sock,
			),
		},
	})
}
