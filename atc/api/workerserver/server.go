package workerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/db"
)

type Server struct {
	logger lager.Logger

	teamFactory     db.TeamFactory
	dbWorkerFactory db.WorkerFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	dbWorkerFactory db.WorkerFactory,

) *Server {
	return &Server{
		logger:          logger,
		teamFactory:     teamFactory,
		dbWorkerFactory: dbWorkerFactory,
	}
}
