package pipelineserver

import (
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	configDB      db.ConfigDB
}

func NewServer(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	configDB db.ConfigDB,
) *Server {
	return &Server{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		configDB:      configDB,
	}
}
