package containerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/gc"
	"github.com/concourse/concourse/v5/atc/worker"
)

type Server struct {
	logger lager.Logger

	workerClient            worker.Client
	secretManager           creds.Secrets
	interceptTimeoutFactory InterceptTimeoutFactory
	containerRepository     db.ContainerRepository
	destroyer               gc.Destroyer
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
	secretManager creds.Secrets,
	interceptTimeoutFactory InterceptTimeoutFactory,
	containerRepository db.ContainerRepository,
	destroyer gc.Destroyer,
) *Server {
	return &Server{
		logger:                  logger,
		workerClient:            workerClient,
		secretManager:           secretManager,
		interceptTimeoutFactory: interceptTimeoutFactory,
		containerRepository:     containerRepository,
		destroyer:               destroyer,
	}
}
