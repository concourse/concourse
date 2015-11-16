package jobserver

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/scheduler"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . SchedulerFactory

type SchedulerFactory interface {
	BuildScheduler(db.PipelineDB) scheduler.BuildScheduler
}

type Server struct {
	logger lager.Logger

	schedulerFactory SchedulerFactory
}

func NewServer(
	logger lager.Logger,
	schedulerFactory SchedulerFactory,
) *Server {
	return &Server{
		logger:           logger,
		schedulerFactory: schedulerFactory,
	}
}
