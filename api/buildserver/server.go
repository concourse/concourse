package buildserver

import (
	"time"

	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/logfanout"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger

	db           BuildsDB
	builder      builder.Builder
	tracker      *logfanout.Tracker
	pingInterval time.Duration
}

type BuildsDB interface {
	CreateOneOffBuild() (builds.Build, error)
}

func NewServer(
	logger lager.Logger,
	db BuildsDB,
	builder builder.Builder,
	tracker *logfanout.Tracker,
	pingInterval time.Duration,
) *Server {
	return &Server{
		logger:       logger,
		db:           db,
		builder:      builder,
		tracker:      tracker,
		pingInterval: pingInterval,
	}
}
