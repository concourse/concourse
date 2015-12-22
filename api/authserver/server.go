package authserver

import (
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger          lager.Logger
	externalURL     string
	tokenGenerator  auth.TokenGenerator
	providerFactory auth.ProviderFactory
	db              AuthDB
}

//go:generate counterfeiter . AuthDB

type AuthDB interface {
	GetTeamByName(teamName string) (db.SavedTeam, bool, error)
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	tokenGenerator auth.TokenGenerator,
	providerFactory auth.ProviderFactory,
	db AuthDB,
) *Server {
	return &Server{
		logger:          logger,
		externalURL:     externalURL,
		tokenGenerator:  tokenGenerator,
		providerFactory: providerFactory,
		db:              db,
	}
}
