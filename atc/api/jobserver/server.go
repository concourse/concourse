package jobserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	externalURL   string
	rejector      auth.Rejector
	secretManager creds.Secrets
	jobFactory    db.JobFactory
	checkFactory  db.CheckFactory
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	secretManager creds.Secrets,
	jobFactory db.JobFactory,
	checkFactory db.CheckFactory,
) *Server {
	return &Server{
		logger:        logger,
		externalURL:   externalURL,
		rejector:      auth.UnauthorizedRejector{},
		secretManager: secretManager,
		jobFactory:    jobFactory,
		checkFactory:  checkFactory,
	}
}

func conflictArchivedHandler(logger lager.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("pipeline-is-archived")
		http.Error(w, "action not allowed for archived pipeline", http.StatusConflict)
	})
}
