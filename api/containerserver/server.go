package containerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/worker"
)

type Server struct {
	logger lager.Logger

	workerClient            worker.Client
	variablesFactory        creds.VariablesFactory
	interceptTimeoutFactory InterceptTimeoutFactory
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
	variablesFactory creds.VariablesFactory,
	interceptTimeoutFactory InterceptTimeoutFactory,
) *Server {
	return &Server{
		logger:                  logger,
		workerClient:            workerClient,
		variablesFactory:        variablesFactory,
		interceptTimeoutFactory: interceptTimeoutFactory,
	}
}
