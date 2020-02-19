package volumeserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/handles"
)

type Server struct {
	logger       lager.Logger
	repository   db.VolumeRepository
	destroyer    gc.Destroyer
	volumeSyncer handles.Syncer
}

func NewServer(
	logger lager.Logger,
	volumeRepository db.VolumeRepository,
	destroyer gc.Destroyer,
	volumeSyncer handles.Syncer,
) *Server {
	return &Server{
		logger:       logger,
		repository:   volumeRepository,
		destroyer:    destroyer,
		volumeSyncer: volumeSyncer,
	}
}
