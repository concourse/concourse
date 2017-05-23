package configserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger      lager.Logger
	teamFactory dbng.TeamFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory dbng.TeamFactory,
) *Server {
	return &Server{
		logger:      logger,
		teamFactory: teamFactory,
	}
}
