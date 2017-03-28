package containerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

type Server struct {
	logger lager.Logger

	workerClient worker.Client

	teamDBFactory db.TeamDBFactory
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
	teamDBFactory db.TeamDBFactory,
) *Server {
	return &Server{
		logger:        logger,
		workerClient:  workerClient,
		teamDBFactory: teamDBFactory,
	}
}
