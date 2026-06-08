package healthserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
)

// Server holds the dependencies needed to serve the health endpoint.
type Server struct {
	logger           lager.Logger
	dbConn           db.DbConn
	workerFactory    db.WorkerFactory
	minWorkerCount   int
}

func NewServer(
	logger lager.Logger,
	dbConn db.DbConn,
	workerFactory db.WorkerFactory,
	minWorkerCount int,
) *Server {
	return &Server{
		logger:         logger,
		dbConn:         dbConn,
		workerFactory:  workerFactory,
		minWorkerCount: minWorkerCount,
	}
}
