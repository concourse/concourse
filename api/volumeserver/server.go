package volumeserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

type Server struct {
	logger  lager.Logger
	factory db.VolumeFactory
}

func NewServer(
	logger lager.Logger,
	vf db.VolumeFactory,
) *Server {
	return &Server{
		logger:  logger,
		factory: vf,
	}
}
