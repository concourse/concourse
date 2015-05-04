package pipelineserver

import (
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger
	db     db.PipelinesDB
}

func NewServer(
	logger lager.Logger,
	db db.PipelinesDB,
) *Server {
	return &Server{
		logger: logger,
		db:     db,
	}
}
