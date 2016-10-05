package workerserver

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger lager.Logger

	db              WorkerDB
	teamDBFactory   db.TeamDBFactory
	dbTeamFactory   dbng.TeamFactory
	dbWorkerFactory dbng.WorkerFactory
}

//go:generate counterfeiter . WorkerDB

type WorkerDB interface {
	SaveWorker(db.WorkerInfo, time.Duration) (db.SavedWorker, error)
	Workers() ([]db.SavedWorker, error)
}

func NewServer(
	logger lager.Logger,
	db WorkerDB,
	teamDBFactory db.TeamDBFactory,
	dbTeamFactory dbng.TeamFactory,
	dbWorkerFactory dbng.WorkerFactory,
) *Server {
	return &Server{
		logger:          logger,
		db:              db,
		teamDBFactory:   teamDBFactory,
		dbTeamFactory:   dbTeamFactory,
		dbWorkerFactory: dbWorkerFactory,
	}
}
