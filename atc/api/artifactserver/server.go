package artifactserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/worker"
)

type Server struct {
	logger     lager.Logger
	workerPool worker.Pool
}

func NewServer(
	logger lager.Logger,
	workerPool worker.Pool,
) *Server {
	return &Server{
		logger:     logger,
		workerPool: workerPool,
	}
}
