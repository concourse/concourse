package versionserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger      lager.Logger
	externalURL string
	resourceConfigScopeFactory db.ResourceConfigScopeFactory
}

func NewServer(logger lager.Logger,
	externalURL string,
	resourceConfigScopeFactory db.ResourceConfigScopeFactory,
) *Server {
	return &Server{
		logger:      logger,
		externalURL: externalURL,
		resourceConfigScopeFactory: resourceConfigScopeFactory,
	}
}
