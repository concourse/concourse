package volumeserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger  lager.Logger
	factory dbng.VolumeFactory
}

func NewServer(
	logger lager.Logger,
	vf dbng.VolumeFactory,
) *Server {
	return &Server{
		logger:  logger,
		factory: vf,
	}
}
