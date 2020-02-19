package containerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/handles"
	"github.com/concourse/concourse/atc/worker"
)

type Server struct {
	logger lager.Logger

	workerClient            worker.Client
	secretManager           creds.Secrets
	varSourcePool           creds.VarSourcePool
	interceptTimeoutFactory InterceptTimeoutFactory
	containerRepository     db.ContainerRepository
	containerSyncer         handles.Syncer
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
	secretManager creds.Secrets,
	varSourcePool creds.VarSourcePool,
	interceptTimeoutFactory InterceptTimeoutFactory,
	containerRepository db.ContainerRepository,
	containerSyncer handles.Syncer,
) *Server {
	return &Server{
		logger:                  logger,
		workerClient:            workerClient,
		secretManager:           secretManager,
		varSourcePool:           varSourcePool,
		interceptTimeoutFactory: interceptTimeoutFactory,
		containerRepository:     containerRepository,
		containerSyncer:         containerSyncer,
	}
}
