package ccserver

import (
	"code.cloudfoundry.org/lager"
)

type Server struct {
	logger           lager.Logger
}

func NewServer(
	logger lager.Logger,
) *Server {
	return &Server{
		logger:           logger,
	}
}
