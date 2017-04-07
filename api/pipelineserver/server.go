package pipelineserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger      lager.Logger
	teamFactory dbng.TeamFactory
	rejector    auth.Rejector
	pipelinesDB db.PipelinesDB
}

func NewServer(
	logger lager.Logger,
	teamFactory dbng.TeamFactory,
	pipelinesDB db.PipelinesDB,
) *Server {
	return &Server{
		logger:      logger,
		teamFactory: teamFactory,
		rejector:    auth.UnauthorizedRejector{},
		pipelinesDB: pipelinesDB,
	}
}
