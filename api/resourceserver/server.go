package resourceserver

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
)

type Server struct {
	logger lager.Logger

	resourceDB ResourceDB
	configDB   ConfigDB
}

type ConfigDB interface {
	GetConfig() (atc.Config, error)
}

//go:generate counterfeiter . ResourceDB

type ResourceDB interface {
	EnableVersionedResource(resourceID int) error
	DisableVersionedResource(resourceID int) error
}

func NewServer(
	logger lager.Logger,
	resourceDB ResourceDB,
	configDB ConfigDB,
) *Server {
	return &Server{
		logger:     logger,
		resourceDB: resourceDB,
		configDB:   configDB,
	}
}
