package jobserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/api/auth"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
)

type Server struct {
	logger lager.Logger

	externalURL   string
	rejector      auth.Rejector
	secretManager creds.Secrets
	jobFactory    db.JobFactory
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	secretManager creds.Secrets,
	jobFactory db.JobFactory,
) *Server {
	return &Server{
		logger:        logger,
		externalURL:   externalURL,
		rejector:      auth.UnauthorizedRejector{},
		secretManager: secretManager,
		jobFactory:    jobFactory,
	}
}
