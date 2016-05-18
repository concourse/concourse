package teamserver

import (
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . TeamsDB

type TeamsDB interface {
	CreateTeam(data db.Team) (db.SavedTeam, error)
}

type Server struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	teamsDB       TeamsDB
}

func NewServer(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	teamsDB TeamsDB,
) *Server {
	return &Server{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		teamsDB:       teamsDB,
	}
}
