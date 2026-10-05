package pipelineserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/api/auth"
	"github.com/concourse/concourse/v8/atc/db"
)

type Server struct {
	logger          lager.Logger
	teamFactory     db.TeamFactory
	rejector        auth.Rejector
	pipelineFactory db.PipelineFactory
	externalURL     string
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	pipelineFactory db.PipelineFactory,
	externalURL string,
) *Server {
	return &Server{
		logger:          logger,
		teamFactory:     teamFactory,
		rejector:        auth.UnauthorizedRejector{},
		pipelineFactory: pipelineFactory,
		externalURL:     externalURL,
	}
}
