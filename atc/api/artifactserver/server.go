package artifactserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/worker"
)

type Server struct {
	logger       lager.Logger
	workerClient worker.Client
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
) *Server {
	return &Server{
		logger:       logger,
		workerClient: workerClient,
	}
}
