package resourceserver

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	resourceDB ResourceDB
	configDB   db.ConfigDB
}

//go:generate counterfeiter . ResourceDB

type ResourceDB interface {
	EnableVersionedResource(resourceID int) error
	DisableVersionedResource(resourceID int) error
}

func NewServer(
	logger lager.Logger,
	resourceDB ResourceDB,
	configDB db.ConfigDB,
) *Server {
	return &Server{
		logger:     logger,
		resourceDB: resourceDB,
		configDB:   configDB,
	}
}
