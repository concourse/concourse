package workerserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/db"
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
