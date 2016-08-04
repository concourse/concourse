package pipelineserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
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
