package healthserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
)

type Server struct {
	logger        lager.Logger
	credsManagers creds.Managers
}

func NewServer(
	logger lager.Logger,
	credsManagers creds.Managers,
) *Server {
	return &Server{
		logger:        logger,
		credsManagers: credsManagers,
	}
}
