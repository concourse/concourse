package configserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	teamFactory   dbng.TeamFactory
}

func NewServer(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	teamFactory dbng.TeamFactory,
) *Server {
	return &Server{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		teamFactory:   teamFactory,
	}
}
