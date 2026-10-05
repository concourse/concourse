package configserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/atc/db"
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
