package configserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	teamFactory   dbng.TeamFactory
	validate      ConfigValidator
}

type ConfigValidator func(atc.Config) ([]config.Warning, []string)

func NewServer(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	teamFactory dbng.TeamFactory,
	validator ConfigValidator,
) *Server {
	return &Server{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		teamFactory:   teamFactory,
		validate:      validator,
	}
}
