package resourceserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger                lager.Logger
	secretManager         creds.Secrets
	checker               Checker
	resourceFactory       db.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
}

func NewServer(
	logger lager.Logger,
	secretManager creds.Secrets,
	checker Checker,
	resourceFactory db.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
) *Server {
	return &Server{
		logger:                logger,
		secretManager:         secretManager,
		checker:               checker,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
	}
}
