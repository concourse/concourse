package containerserver

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/worker"
)

type Server struct {
	logger lager.Logger

	workerClient         worker.Client
	variablesFactory     creds.VariablesFactory
	interceptIdleTimeout time.Duration
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
	variablesFactory creds.VariablesFactory,
	interceptIdleTimeout time.Duration,
) *Server {
	return &Server{
		logger:               logger,
		workerClient:         workerClient,
		variablesFactory:     variablesFactory,
		interceptIdleTimeout: interceptIdleTimeout,
	}
}
