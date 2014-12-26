package buildserver

import (
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/pivotal-golang/lager"
)

type EventHandlerFactory func(BuildsDB, int, engine.Engine, bool) http.Handler

type Server struct {
	logger lager.Logger

	engine              engine.Engine
	db                  BuildsDB
	configDB            ConfigDB
	pingInterval        time.Duration
	eventHandlerFactory EventHandlerFactory
	drain               <-chan struct{}
	fallback            auth.Validator

	httpClient *http.Client
}

//go:generate counterfeiter . BuildsDB
type BuildsDB interface {
	GetBuild(buildID int) (db.Build, error)
	GetAllBuilds() ([]db.Build, error)

	CreateOneOffBuild() (db.Build, error)
}

//go:generate counterfeiter . ConfigDB
type ConfigDB interface {
	GetConfig() (atc.Config, error)
}

func NewServer(
	logger lager.Logger,
	engine engine.Engine,
	db BuildsDB,
	configDB ConfigDB,
	pingInterval time.Duration,
	eventHandlerFactory EventHandlerFactory,
	drain <-chan struct{},
	fallback auth.Validator,
) *Server {
	return &Server{
		logger:              logger,
		engine:              engine,
		db:                  db,
		configDB:            configDB,
		pingInterval:        pingInterval,
		eventHandlerFactory: eventHandlerFactory,
		fallback:            fallback,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}
