package jobserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	externalURL      string
	rejector         auth.Rejector
	variablesFactory creds.VariablesFactory
	jobFactory       db.JobFactory
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	variablesFactory creds.VariablesFactory,
	jobFactory db.JobFactory,
) *Server {
	return &Server{
		logger:           logger,
		externalURL:      externalURL,
		rejector:         auth.UnauthorizedRejector{},
		variablesFactory: variablesFactory,
		jobFactory:       jobFactory,
	}
}
