package jobserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"net/http"
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

func (s *Server) logIfErrorAndRespond(err error, w http.ResponseWriter, message string, statusCode int) bool {
	if err != nil {
		s.logger.Error(message, err)
		w.WriteHeader(statusCode)
		return true
	}
	return false
}

func (s *Server) checkResultAndRespond(result bool, w http.ResponseWriter, statusCode int) bool {
	if result {
		w.WriteHeader(statusCode)
		return true
	}
	return false
}
