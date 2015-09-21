package buildserver

import (
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type EventHandlerFactory func(BuildsDB, int) http.Handler

type Server struct {
	logger lager.Logger

	engine              engine.Engine
	workerClient        worker.Client
	db                  BuildsDB
	configDB            db.ConfigDB
	eventHandlerFactory EventHandlerFactory
	drain               <-chan struct{}
	fallback            auth.Validator

	httpClient *http.Client
}

//go:generate counterfeiter . BuildsDB
type BuildsDB interface {
	GetBuild(buildID int) (db.Build, bool, error)
	GetBuildEvents(buildID int, from uint) (db.EventSource, error)

	GetAllBuilds() ([]db.Build, error)

	CreateOneOffBuild() (db.Build, error)
	GetConfigByBuildID(buildID int) (atc.Config, db.ConfigVersion, error)
}

func NewServer(
	logger lager.Logger,
	engine engine.Engine,
	workerClient worker.Client,
	db BuildsDB,
	configDB db.ConfigDB,
	eventHandlerFactory EventHandlerFactory,
	drain <-chan struct{},
	fallback auth.Validator,
) *Server {
	return &Server{
		logger:              logger,
		engine:              engine,
		workerClient:        workerClient,
		db:                  db,
		configDB:            configDB,
		eventHandlerFactory: eventHandlerFactory,
		drain:               drain,
		fallback:            fallback,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}
