package versionserver

import "code.cloudfoundry.org/lager"

type Server struct {
	logger      lager.Logger
	externalURL string
}

func NewServer(logger lager.Logger, externalURL string) *Server {
	return &Server{
		logger:      logger,
		externalURL: externalURL,
	}
}
