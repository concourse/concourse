package resourceserver

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	resourceDB ResourceDB
	configDB   db.ConfigDB

	validator auth.Validator
}

//go:generate counterfeiter . ResourceDB

type ResourceDB interface {
	EnableVersionedResource(resourceID int) error
	DisableVersionedResource(resourceID int) error

	GetResource(resourceName string) (db.Resource, error)
	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error
}

func NewServer(
	logger lager.Logger,
	resourceDB ResourceDB,
	configDB db.ConfigDB,
	validator auth.Validator,
) *Server {
	return &Server{
		logger:     logger,
		resourceDB: resourceDB,
		configDB:   configDB,
		validator:  validator,
	}
}
