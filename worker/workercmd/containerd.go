package workercmd

import (
	"fmt"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd"
	"github.com/tedsuo/ifrit"
)

func (cmd *WorkerCommand) containerdRunner(logger lager.Logger) (ifrit.Runner, error) {
	client, err := libcontainerd.New("address")
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate containerd client: %w", err)
	}

	backend := backend.New(client)

	server := server.New(
		"tcp",
		cmd.bindAddr(),
		0,
		&backend,
		logger,
	)

	return gardenServerRunner{logger, server}, nil
}
