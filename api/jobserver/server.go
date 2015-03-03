package jobserver

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	db       JobsDB
	configDB db.ConfigDB
}

//go:generate counterfeiter . JobsDB
type JobsDB interface {
	GetAllJobBuilds(job string) ([]db.Build, error)
	GetCurrentBuild(job string) (db.Build, error)
	GetJobBuild(job string, build string) (db.Build, error)
	GetJobFinishedAndNextBuild(job string) (*db.Build, *db.Build, error)
}

func NewServer(
	logger lager.Logger,
	db JobsDB,
	configDB db.ConfigDB,
) *Server {
	return &Server{
		logger:   logger,
		db:       db,
		configDB: configDB,
	}
}
