package resourceserver

import "github.com/pivotal-golang/lager"

type Server struct {
	logger lager.Logger
}

func NewServer(logger lager.Logger) *Server {
	return &Server{
		logger: logger,
	}
}
