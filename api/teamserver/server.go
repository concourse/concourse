package teamserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . TeamsDB

type TeamsDB interface {
	GetTeams() ([]db.SavedTeam, error)
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
