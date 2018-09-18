package workerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

type Server struct {
	logger lager.Logger

	teamFactory     db.TeamFactory
	dbWorkerFactory db.WorkerFactory
	workerProvider  worker.WorkerProvider
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	dbWorkerFactory db.WorkerFactory,
	workerProvider worker.WorkerProvider,

) *Server {
	return &Server{
		logger:          logger,
		teamFactory:     teamFactory,
		dbWorkerFactory: dbWorkerFactory,
		workerProvider:  workerProvider,
	}
}
