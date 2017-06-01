package pipelineserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger          lager.Logger
	teamFactory     dbng.TeamFactory
	rejector        auth.Rejector
	pipelineFactory dbng.PipelineFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory dbng.TeamFactory,
	pipelineFactory dbng.PipelineFactory,
) *Server {
	return &Server{
		logger:          logger,
		teamFactory:     teamFactory,
		rejector:        auth.UnauthorizedRejector{},
		pipelineFactory: pipelineFactory,
	}
}
