package workerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	teamFactory         db.TeamFactory
	dbWorkerFactory     db.WorkerFactory
	containerRepository db.ContainerRepository
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	dbWorkerFactory db.WorkerFactory,
	containerRepository db.ContainerRepository,
) *Server {
	return &Server{
		logger:              logger,
		teamFactory:         teamFactory,
		dbWorkerFactory:     dbWorkerFactory,
		containerRepository: containerRepository,
	}
}
