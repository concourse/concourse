package volumeserver

import (
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger

	db VolumesDB
}

//go:generate counterfeiter . VolumesDB

type VolumesDB interface {
	GetVolumes() ([]db.SavedVolumeData, error)
}

func NewServer(
	logger lager.Logger,
	db VolumesDB,
) *Server {
	return &Server{
		logger: logger,
		db:     db,
	}
}
