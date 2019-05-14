package infoserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
)

type Server struct {
	logger        lager.Logger
	version       string
	workerVersion string
	externalURL   string
	clusterName   string
	credsManagers creds.Managers
}

func NewServer(
	logger lager.Logger,
	version string,
	workerVersion string,
	externalURL string,
	clusterName string,
	credsManagers creds.Managers,
) *Server {
	return &Server{
		logger:        logger,
		version:       version,
		workerVersion: workerVersion,
		externalURL:   externalURL,
		clusterName:   clusterName,
		credsManagers: credsManagers,
	}
}
