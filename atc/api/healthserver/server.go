package healthserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
)

// Server holds the dependencies needed to serve the health endpoint.
type Server struct {
	logger                   lager.Logger
	dbConn                   db.DbConn
	workerFactory            db.WorkerFactory
	componentFactory         db.ComponentFactory
	minWorkerCount           int
	componentStaleMultiplier float64
}

func NewServer(
	logger lager.Logger,
	dbConn db.DbConn,
	workerFactory db.WorkerFactory,
	componentFactory db.ComponentFactory,
	minWorkerCount int,
	componentStaleMultiplier float64,
) *Server {
	return &Server{
		logger:                   logger,
		dbConn:                   dbConn,
		workerFactory:            workerFactory,
		componentFactory:         componentFactory,
		minWorkerCount:           minWorkerCount,
		componentStaleMultiplier: componentStaleMultiplier,
	}
}
