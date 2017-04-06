package authserver

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type Server struct {
	logger             lager.Logger
	externalURL        string
	oAuthBaseURL       string
	authTokenGenerator auth.AuthTokenGenerator
	csrfTokenGenerator auth.CSRFTokenGenerator
	providerFactory    auth.ProviderFactory
	teamDBFactory      db.TeamDBFactory
	teamFactory        dbng.TeamFactory
	expire             time.Duration
	isTLSEnabled       bool
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	oAuthBaseURL string,
	authTokenGenerator auth.AuthTokenGenerator,
	csrfTokenGenerator auth.CSRFTokenGenerator,
	providerFactory auth.ProviderFactory,
	teamDBFactory db.TeamDBFactory,
	teamFactory dbng.TeamFactory,
	expire time.Duration,
	isTLSEnabled bool,
) *Server {
	return &Server{
		logger:             logger,
		externalURL:        externalURL,
		oAuthBaseURL:       oAuthBaseURL,
		authTokenGenerator: authTokenGenerator,
		csrfTokenGenerator: csrfTokenGenerator,
		providerFactory:    providerFactory,
		teamDBFactory:      teamDBFactory,
		teamFactory:        teamFactory,
		expire:             expire,
		isTLSEnabled:       isTLSEnabled,
	}
}
