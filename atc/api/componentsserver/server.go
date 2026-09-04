package componentsserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/db"
)

type Server struct {
	logger           lager.Logger
	componentFactory db.ComponentFactory
}

func NewServer(
	logger lager.Logger,
	comp db.ComponentFactory,
) *Server {
	return &Server{
		logger:           logger,
		componentFactory: comp,
	}
}
