package workercmd

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"github.com/vito/houdini"
)

func (cmd *WorkerCommand) houdiniRunner(logger lager.Logger) (ifrit.Runner, error) {
	depotDir := filepath.Join(cmd.WorkDir.Path(), "containers")

	err := os.MkdirAll(depotDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create depot dir: %s", err)
	}

	backend := houdini.NewBackend(depotDir)

	server := server.New(
		"tcp",
		cmd.bindAddr(),
		0,
		backend,
		logger,
	)

	return gardenServerRunner{logger, server}, nil
}

func (cmd *WorkerCommand) bindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP.IP, cmd.BindPort)
}
