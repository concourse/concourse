package workercmd

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/houdini"
	"github.com/tedsuo/ifrit"
)

func (cmd *WorkerCommand) houdiniRunner(logger lager.Logger) (ifrit.Runner, error) {
	depotDir := filepath.Join(cmd.WorkDir.Path(), "containers")

	err := os.MkdirAll(depotDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create depot dir: %s", err)
	}

	backend := houdini.NewBackend(depotDir)

	return newGardenServerRunner(
		"tcp",
		cmd.bindAddr(),
		0,
		backend,
		logger,
	), nil
}

func (cmd *WorkerCommand) bindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP.IP, cmd.BindPort)
}
