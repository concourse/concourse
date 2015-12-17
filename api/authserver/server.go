package authserver

import (
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/provider"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger           lager.Logger
	externalURL      string
	tokenGenerator   auth.TokenGenerator
	providers        provider.Providers
	basicAuthEnabled bool
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	tokenGenerator auth.TokenGenerator,
	providers provider.Providers,
	basicAuthEnabled bool,
) *Server {
	return &Server{
		logger:           logger,
		externalURL:      externalURL,
		tokenGenerator:   tokenGenerator,
		providers:        providers,
		basicAuthEnabled: basicAuthEnabled,
	}
}
