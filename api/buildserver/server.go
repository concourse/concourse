package buildserver

import (
	"net/http"
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

	httpClient *http.Client
}

type BuildsDB interface {
	GetBuild(buildID int) (builds.Build, error)
	GetAllBuilds() ([]builds.Build, error)

	CreateOneOffBuild() (builds.Build, error)
	AbortBuild(buildID int) (string, error)
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

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}
