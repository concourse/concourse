package pipelineserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/api/auth"
	"github.com/concourse/concourse/v5/atc/db"
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
