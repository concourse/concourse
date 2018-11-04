package ccserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger           lager.Logger
	teamFactory      db.TeamFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
) *Server {
	return &Server{
		logger:           logger,
		teamFactory:      teamFactory,
	}
}
