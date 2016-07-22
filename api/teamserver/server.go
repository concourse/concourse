package teamserver

import (
	"time"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . TeamDB

type TeamDB interface {
	CreateTeam(data db.Team) (db.SavedTeam, error)
	SaveWorker(info db.WorkerInfo, ttl time.Duration) (db.SavedWorker, error)
}

type Server struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	teamDB        TeamDB
}

func NewServer(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	teamDB TeamDB,
) *Server {
	return &Server{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		teamDB:        teamDB,
	}
}
