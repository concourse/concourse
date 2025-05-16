package idtokenserver

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
)

// Server handles the requests related to the idtoken credential provider
// Most importantly it publishes the public signing keys and offers a way to discover them
type Server struct {
	logger              lager.Logger
	externalURL         string
	dbSigningKeyFactory db.SigningKeyFactory
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	dbSigningKeyFactory db.SigningKeyFactory,
) *Server {
	return &Server{
		logger:              logger,
		externalURL:         externalURL,
		dbSigningKeyFactory: dbSigningKeyFactory,
	}
}
