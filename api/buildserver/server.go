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

type EventHandlerFactory func(lager.Logger, BuildsDB, int) http.Handler

type Server struct {
	logger lager.Logger

	externalURL string

	engine              engine.Engine
	workerClient        worker.Client
	db                  BuildsDB
	teamDBFactory       db.TeamDBFactory
	eventHandlerFactory EventHandlerFactory
	drain               <-chan struct{}
	rejector            auth.Rejector

	httpClient *http.Client
}

//go:generate counterfeiter . BuildsDB

type BuildsDB interface {
	GetBuild(buildID int) (db.Build, bool, error)
	GetBuildEvents(buildID int, from uint) (db.EventSource, error)
	GetBuildResources(buildID int) ([]db.BuildInput, []db.BuildOutput, error)
	GetBuildPreparation(buildID int) (db.BuildPreparation, bool, error)

	GetBuilds(teamName string, page db.Page) ([]db.Build, db.Pagination, error)

	GetConfigByBuildID(buildID int) (atc.Config, db.ConfigVersion, error)
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	engine engine.Engine,
	workerClient worker.Client,
	db BuildsDB,
	teamDBFactory db.TeamDBFactory,
	eventHandlerFactory EventHandlerFactory,
	drain <-chan struct{},
) *Server {
	return &Server{
		logger: logger,

		externalURL: externalURL,

		engine:              engine,
		workerClient:        workerClient,
		db:                  db,
		teamDBFactory:       teamDBFactory,
		eventHandlerFactory: eventHandlerFactory,
		drain:               drain,

		rejector: auth.UnauthorizedRejector{},

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}
