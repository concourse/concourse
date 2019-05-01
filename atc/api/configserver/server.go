package configserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger        lager.Logger
	teamFactory   db.TeamFactory
	secretManager creds.Secrets
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	secretManager creds.Secrets,
) *Server {
	return &Server{
		logger:        logger,
		teamFactory:   teamFactory,
		secretManager: secretManager,
	}
}
