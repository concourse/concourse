package jobserver

import (
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/scheduler"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . SchedulerFactory

type SchedulerFactory interface {
	BuildScheduler(db.PipelineDB, string) scheduler.BuildScheduler
}

type Server struct {
	logger lager.Logger

	schedulerFactory SchedulerFactory
	externalURL      string
	rejector         auth.Rejector
}

func NewServer(
	logger lager.Logger,
	schedulerFactory SchedulerFactory,
	externalURL string,
) *Server {
	return &Server{
		logger:           logger,
		schedulerFactory: schedulerFactory,
		externalURL:      externalURL,
		rejector:         auth.UnauthorizedRejector{},
	}
}
