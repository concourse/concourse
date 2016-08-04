package infoserver

import "code.cloudfoundry.org/lager"

type Server struct {
	logger  lager.Logger
	version string
}

func NewServer(
	logger lager.Logger,
	version string,
) *Server {
	return &Server{
		logger:  logger,
		version: version,
	}
}
