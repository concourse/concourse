package loglevelserver

import "code.cloudfoundry.org/lager/v3"

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
