package workerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger lager.Logger

	teamDBFactory   db.TeamDBFactory
	dbTeamFactory   dbng.TeamFactory
	dbWorkerFactory dbng.WorkerFactory
}

func NewServer(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	dbTeamFactory dbng.TeamFactory,
	dbWorkerFactory dbng.WorkerFactory,
) *Server {
	return &Server{
		logger:          logger,
		teamDBFactory:   teamDBFactory,
		dbTeamFactory:   dbTeamFactory,
		dbWorkerFactory: dbWorkerFactory,
	}
}
