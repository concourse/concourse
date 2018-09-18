package configserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger           lager.Logger
	teamFactory      db.TeamFactory
	variablesFactory creds.VariablesFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	variablesFactory creds.VariablesFactory,
) *Server {
	return &Server{
		logger:           logger,
		teamFactory:      teamFactory,
		variablesFactory: variablesFactory,
	}
}
