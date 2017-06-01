package workerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger lager.Logger

	teamFactory     dbng.TeamFactory
	dbWorkerFactory dbng.WorkerFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory dbng.TeamFactory,
	dbWorkerFactory dbng.WorkerFactory,
) *Server {
	return &Server{
		logger:          logger,
		teamFactory:     teamFactory,
		dbWorkerFactory: dbWorkerFactory,
	}
}
