package pipelineserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger          lager.Logger
	teamFactory     dbng.TeamFactory
	teamDBFactory   db.TeamDBFactory
	rejector        auth.Rejector
	pipelineFactory dbng.PipelineFactory
}

func NewServer(
	logger lager.Logger,
	teamFactory dbng.TeamFactory,
	teamDBFactory db.TeamDBFactory,
	pipelineFactory dbng.PipelineFactory,
) *Server {
	return &Server{
		logger:          logger,
		teamFactory:     teamFactory,
		teamDBFactory:   teamDBFactory,
		rejector:        auth.UnauthorizedRejector{},
		pipelineFactory: pipelineFactory,
	}
}
