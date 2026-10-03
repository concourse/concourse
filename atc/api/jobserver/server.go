package jobserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/api/auth"
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/atc/db"
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
