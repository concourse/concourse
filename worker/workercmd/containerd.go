package workercmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	concourseCmd "github.com/concourse/concourse/cmd"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

func containerdGardenServer(logger lager.Logger, bindAddr, containerdAddr string) *server.GardenServer {
	backend := backend.New(libcontainerd.New(containerdAddr))

	return server.New(
		"tcp",
		bindAddr,
		0,
		&backend,
		logger,
	)
}

// TODO for gdn server, use a `restarter`?
//
func (cmd *WorkerCommand) containerdRunner(logger lager.Logger) (ifrit.Runner, error) {
	// set PATH accordingly
	//
	// TODO - should this be done elsewhere?
	//
	if binDir := concourseCmd.DiscoverAsset("bin"); binDir != "" {
		err := os.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
		if err != nil {
			return nil, err
		}
	}

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

	return grouper.NewOrdered(os.Interrupt, grouper.Members{
		{
			Name:   "containerd",
			Runner: cmdRunner{cmd: containerdCmd},
		},
		{
			Name: "containerd-backend",
			Runner: gardenServerRunner{
				logger,
				containerdGardenServer(logger, cmd.bindAddr(), containerdSock),
			},
		},
	}), nil
}
