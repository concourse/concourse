package buildserver

import (
	"net/http"
	"time"

	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/pivotal-golang/lager"
)

type EventHandlerFactory func(event.BuildsDB, int, event.Censor) http.Handler

type Server struct {
	logger lager.Logger

	db                  BuildsDB
	builder             builder.Builder
	pingInterval        time.Duration
	eventHandlerFactory EventHandlerFactory
	drain               <-chan struct{}

	httpClient *http.Client
}

type BuildsDB interface {
	GetBuild(buildID int) (builds.Build, error)
	GetAllBuilds() ([]builds.Build, error)

	CreateOneOffBuild() (builds.Build, error)
	AbortBuild(buildID int) error

	BuildEvents(buildID int) ([]db.BuildEvent, error)
}

func NewServer(
	logger lager.Logger,
	db BuildsDB,
	builder builder.Builder,
	pingInterval time.Duration,
	eventHandlerFactory EventHandlerFactory,
	drain <-chan struct{},
) *Server {
	return &Server{
		logger:              logger,
		db:                  db,
		builder:             builder,
		pingInterval:        pingInterval,
		eventHandlerFactory: eventHandlerFactory,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}
