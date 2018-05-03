package containerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/gc"
	"github.com/concourse/atc/worker"
)

type Server struct {
	logger lager.Logger

	workerClient            worker.Client
	variablesFactory        creds.VariablesFactory
	interceptTimeoutFactory InterceptTimeoutFactory
	containerRepository     db.ContainerRepository
	containerDestroyer      gc.ContainerDestroyer
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
	variablesFactory creds.VariablesFactory,
	interceptTimeoutFactory InterceptTimeoutFactory,
	containerRepository db.ContainerRepository,
	containerDestroyer gc.ContainerDestroyer,
) *Server {
	return &Server{
		logger:                  logger,
		workerClient:            workerClient,
		variablesFactory:        variablesFactory,
		interceptTimeoutFactory: interceptTimeoutFactory,
		containerRepository:     containerRepository,
		containerDestroyer:      containerDestroyer,
	}
}
