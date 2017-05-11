package infoserver

import "code.cloudfoundry.org/lager"

type Server struct {
	logger        lager.Logger
	version       string
	workerVersion string
}

func NewServer(
	logger lager.Logger,
	version string,
	workerVersion string,
) *Server {
	return &Server{
		logger:        logger,
		version:       version,
		workerVersion: workerVersion,
	}
}
