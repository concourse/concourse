package loglevelserver

import "code.cloudfoundry.org/lager"

type Server struct {
	logger lager.Logger

	sink *lager.ReconfigurableSink
}

func NewServer(logger lager.Logger, sink *lager.ReconfigurableSink) *Server {
	return &Server{
		logger: logger,

		sink: sink,
	}
}
