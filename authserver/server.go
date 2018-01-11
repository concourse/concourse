package authserver

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/auth"
)

type Server struct {
	logger             lager.Logger
	externalURL        string
	oAuthBaseURL       string
	expire             time.Duration
	isTLSEnabled       bool
	teamFactory        db.TeamFactory
	providerFactory    auth.ProviderFactory
	csrfTokenGenerator auth.CSRFTokenGenerator
	authTokenGenerator auth.AuthTokenGenerator
	tokenReader        auth.TokenReader
	tokenValidator     auth.TokenValidator
	basicAuthValidator auth.TokenValidator
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	oAuthBaseURL string,
	expire time.Duration,
	isTLSEnabled bool,
	teamFactory db.TeamFactory,
	providerFactory auth.ProviderFactory,
	csrfTokenGenerator auth.CSRFTokenGenerator,
	authTokenGenerator auth.AuthTokenGenerator,
	tokenReader auth.TokenReader,
	tokenValidator auth.TokenValidator,
	basicAuthValidator auth.TokenValidator,
) *Server {

	return &Server{
		logger:             logger,
		externalURL:        externalURL,
		oAuthBaseURL:       oAuthBaseURL,
		expire:             expire,
		isTLSEnabled:       isTLSEnabled,
		teamFactory:        teamFactory,
		providerFactory:    providerFactory,
		csrfTokenGenerator: csrfTokenGenerator,
		authTokenGenerator: authTokenGenerator,
		tokenReader:        tokenReader,
		tokenValidator:     tokenValidator,
		basicAuthValidator: basicAuthValidator,
	}
}
