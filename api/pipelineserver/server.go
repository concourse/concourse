package pipelineserver

import (
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger      lager.Logger
	pipelinesDB db.PipelinesDB
	configDB    db.ConfigDB
}

func NewServer(
	logger lager.Logger,
	pipelinesDB db.PipelinesDB,
	configDB db.ConfigDB,
) *Server {
	return &Server{
		logger:      logger,
		pipelinesDB: pipelinesDB,
		configDB:    configDB,
	}
}
