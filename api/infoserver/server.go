package infoserver

import "github.com/pivotal-golang/lager"

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
