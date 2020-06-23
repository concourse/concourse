package jobserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	externalURL        string
	rejector           auth.Rejector
	secretManager      creds.Secrets
	jobFactory         db.JobFactory
	checkFactory       db.CheckFactory
	listAllJobsWatcher ListAllJobsWatcher
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	secretManager creds.Secrets,
	jobFactory db.JobFactory,
	checkFactory db.CheckFactory,
	listAllJobsWatcher ListAllJobsWatcher,
) *Server {
	return &Server{
		logger:             logger,
		externalURL:        externalURL,
		rejector:           auth.UnauthorizedRejector{},
		secretManager:      secretManager,
		jobFactory:         jobFactory,
		checkFactory:       checkFactory,
		listAllJobsWatcher: listAllJobsWatcher,
	}
}
