package authserver

import (
	"github.com/concourse/atc/auth"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger           lager.Logger
	externalURL      string
	tokenGenerator   auth.TokenGenerator
	providers        auth.Providers
	basicAuthEnabled bool
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	tokenGenerator auth.TokenGenerator,
	providers auth.Providers,
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
