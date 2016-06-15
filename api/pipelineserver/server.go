package pipelineserver

import (
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	rejector      auth.Rejector
}

func NewServer(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
) *Server {
	return &Server{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		rejector:      auth.UnauthorizedRejector{},
	}
}
