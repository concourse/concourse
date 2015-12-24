package teamserver

import (
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger
	db     TeamDB
}

//go:generate counterfeiter . TeamDB

type TeamDB interface {
	GetTeamByName(teamName string) (db.SavedTeam, bool, error)
	SaveTeam(team db.Team) (db.SavedTeam, error)
	UpdateTeamBasicAuth(team db.Team) (db.SavedTeam, error)
	UpdateTeamGitHubAuth(team db.Team) (db.SavedTeam, error)
}

func NewServer(
	logger lager.Logger,
	db TeamDB,
) *Server {
	return &Server{
		logger: logger,
		db:     db,
	}
}
