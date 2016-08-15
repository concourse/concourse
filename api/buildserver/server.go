package buildserver

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/worker"
)

type EventHandlerFactory func(lager.Logger, db.Build) http.Handler

//go:generate counterfeiter . BuildsDB

type BuildsDB interface {
	GetPublicBuilds(page db.Page) ([]db.Build, db.Pagination, error)
}

type Server struct {
	logger lager.Logger

	externalURL string

	engine              engine.Engine
	workerClient        worker.Client
	teamDBFactory       db.TeamDBFactory
	buildsDB            BuildsDB
	eventHandlerFactory EventHandlerFactory
	drain               <-chan struct{}
	rejector            auth.Rejector

	httpClient *http.Client
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	engine engine.Engine,
	workerClient worker.Client,
	teamDBFactory db.TeamDBFactory,
	buildsDB BuildsDB,
	eventHandlerFactory EventHandlerFactory,
	drain <-chan struct{},
) *Server {
	return &Server{
		logger: logger,

		externalURL: externalURL,

		engine:              engine,
		workerClient:        workerClient,
		teamDBFactory:       teamDBFactory,
		buildsDB:            buildsDB,
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
