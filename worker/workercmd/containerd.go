package workercmd

import (
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/backend"
	"github.com/tedsuo/ifrit"
)

func (cmd *WorkerCommand) containerdRunner(logger lager.Logger) (ifrit.Runner, error) {
	// [cc] pass address to the socket, or anything else that's needed?
	//
	backend := backend.New()

	server := server.New(
		"tcp",
		cmd.bindAddr(),
		0,
		&backend,
		logger,
	)

	return gardenServerRunner{logger, server}, nil
}
