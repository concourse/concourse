package healthserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/db"
)

type Server struct {
	logger                   lager.Logger
	dbConn                   db.DbConn
	workerFactory            db.WorkerFactory
	componentFactory         db.ComponentFactory
	minWorkerCount           int
	minWorkerThresholds      MinWorkerThresholds
	componentStaleMultiplier float64
}

func NewServer(
	logger lager.Logger,
	dbConn db.DbConn,
	workerFactory db.WorkerFactory,
	componentFactory db.ComponentFactory,
	minWorkerCount int,
	minWorkerThresholds MinWorkerThresholds,
	componentStaleMultiplier float64,
) *Server {
	return &Server{
		logger:                   logger,
		dbConn:                   dbConn,
		workerFactory:            workerFactory,
		componentFactory:         componentFactory,
		minWorkerCount:           minWorkerCount,
		minWorkerThresholds:      minWorkerThresholds,
		componentStaleMultiplier: componentStaleMultiplier,
	}
}
