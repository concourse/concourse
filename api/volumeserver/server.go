package volumeserver

import (
	"github.com/concourse/atc/db"
	"code.cloudfoundry.org/lager"
)

type Server struct {
	logger lager.Logger

	db            VolumesDB
	teamDBFactory db.TeamDBFactory
}

//go:generate counterfeiter . VolumesDB

type VolumesDB interface {
	GetVolumes() ([]db.SavedVolume, error)
}

func NewServer(
	logger lager.Logger,
	db VolumesDB,
	teamDBFactory db.TeamDBFactory,
) *Server {
	return &Server{
		logger:        logger,
		db:            db,
		teamDBFactory: teamDBFactory,
	}
}
