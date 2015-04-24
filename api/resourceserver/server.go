package resourceserver

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/auth"
)

type Server struct {
	logger lager.Logger

	validator auth.Validator
}

func NewServer(
	logger lager.Logger,
	validator auth.Validator,
) *Server {
	return &Server{
		logger:    logger,
		validator: validator,
	}
}
