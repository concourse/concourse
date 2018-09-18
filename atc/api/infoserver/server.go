package infoserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
)

type Server struct {
	logger        lager.Logger
	version       string
	workerVersion string
	credsManagers creds.Managers
}

func NewServer(
	logger lager.Logger,
	version string,
	workerVersion string,
	credsManagers creds.Managers,
) *Server {
	return &Server{
		logger:        logger,
		version:       version,
		workerVersion: workerVersion,
		credsManagers: credsManagers,
	}
}
