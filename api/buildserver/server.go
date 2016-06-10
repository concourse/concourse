package buildserver

import (
	"net/http"
	"time"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type EventHandlerFactory func(lager.Logger, db.Build) http.Handler

type Server struct {
	logger lager.Logger

	externalURL string

	engine              engine.Engine
	workerClient        worker.Client
	teamDBFactory       db.TeamDBFactory
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
	eventHandlerFactory EventHandlerFactory,
	drain <-chan struct{},
) *Server {
	return &Server{
		logger: logger,

		externalURL: externalURL,

		engine:              engine,
		workerClient:        workerClient,
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
