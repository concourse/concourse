package healthserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
)

type Server struct {
	logger           lager.Logger
	variablesFactory creds.VariablesFactory
}

func NewServer(
	logger lager.Logger,
	variablesFactory creds.VariablesFactory,
) *Server {
	return &Server{
		logger:           logger,
		variablesFactory: variablesFactory,
	}
}
