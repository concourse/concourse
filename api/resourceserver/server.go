package resourceserver

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
)

type Server struct {
	logger lager.Logger

	configDB ConfigDB
}

type ConfigDB interface {
	GetConfig() (atc.Config, error)
}

func NewServer(
	logger lager.Logger,
	configDB ConfigDB,
) *Server {
	return &Server{
		logger:   logger,
		configDB: configDB,
	}
}
