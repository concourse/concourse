package versionserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/present"
)

type Server struct {
	logger      lager.Logger
	externalURL string
	router      present.PathBuilder
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	router present.PathBuilder,
) *Server {
	return &Server{
		logger:      logger,
		externalURL: externalURL,
		router:      router,
	}
}
