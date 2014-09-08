package jobserver

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/builds"
)

type Server struct {
	logger lager.Logger

	db JobsDB
}

type JobsDB interface {
	GetJobBuild(job string, build string) (builds.Build, error)
	GetAllJobBuilds(job string) ([]builds.Build, error)
}

func NewServer(
	logger lager.Logger,
	db JobsDB,
) *Server {
	return &Server{
		logger: logger,
		db:     db,
	}
}
