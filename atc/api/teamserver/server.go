package teamserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger      lager.Logger
	teamFactory db.TeamFactory
	externalURL string
	router      present.PathBuilder
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	externalURL string,
	router present.PathBuilder,
) *Server {
	return &Server{
		logger:      logger,
		teamFactory: teamFactory,
		externalURL: externalURL,
		router:      router,
	}
}
