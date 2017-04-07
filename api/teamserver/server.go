package teamserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . TeamsDB

type TeamsDB interface {
	GetTeams() ([]db.SavedTeam, error)
	DeleteTeamByName(teamName string) error
}

type Server struct {
	logger      lager.Logger
	teamsDB     TeamsDB
	teamFactory dbng.TeamFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory dbng.TeamFactory,
	teamsDB TeamsDB,
) *Server {
	return &Server{
		logger:      logger,
		teamFactory: teamFactory,
		teamsDB:     teamsDB,
	}
}
