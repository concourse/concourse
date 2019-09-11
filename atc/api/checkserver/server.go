package checkserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger lager.Logger

	checkFactory db.CheckFactory
}

func NewServer(
	logger lager.Logger,
	checkFactory db.CheckFactory,
) *Server {
	return &Server{
		logger: logger,

		checkFactory: checkFactory,
	}
}
