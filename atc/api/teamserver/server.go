package teamserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

type Server struct {
	logger      lager.Logger
	teamFactory db.TeamFactory
	externalURL string
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	externalURL string,
) *Server {
	return &Server{
		logger:      logger,
		teamFactory: teamFactory,
		externalURL: externalURL,
	}
}
