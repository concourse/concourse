package healthserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger        lager.Logger
	dbWorkerFactory db.WorkerFactory
}

func NewServer(
	logger lager.Logger,
	dbWorkerFactory db.WorkerFactory,
) *Server {
	return &Server{
		logger:        logger,
		dbWorkerFactory: dbWorkerFactory,
	}
}
