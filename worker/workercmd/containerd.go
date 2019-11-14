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
	const graceTime = 0

	backend := backend.New(libcontainerd.New(containerdAddr))

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

func (cmd *WorkerCommand) containerdRunner(logger lager.Logger) (ifrit.Runner, error) {
	containerdSock := filepath.Join(cmd.WorkDir.Path(), "containerd.sock")

	containerdArgs := []string{
		"--address", containerdSock,
		"--root", filepath.Join(cmd.WorkDir.Path(), "containerd"),
	}

	containerdCmd := exec.Command("containerd", containerdArgs...)
	containerdCmd.Stdout = os.Stdout
	containerdCmd.Stderr = os.Stderr
	containerdCmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	return grouper.NewParallel(os.Interrupt, grouper.Members{
		{
			Name:   "containerd",
			Runner: cmdRunner{cmd: containerdCmd},
		},
		{
			Name: "containerd-backend",
			Runner: containerdGardenServerRunner(
				logger, cmd.bindAddr(), containerdSock,
			),
		},
	}), nil
}
