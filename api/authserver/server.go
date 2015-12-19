package authserver

import (
	"github.com/concourse/atc/auth"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger           lager.Logger
	externalURL      string
	tokenGenerator   auth.TokenGenerator
	providerFactory  auth.ProviderFactory
	basicAuthEnabled bool
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	tokenGenerator auth.TokenGenerator,
	providerFactory auth.ProviderFactory,
	basicAuthEnabled bool,
) *Server {
	return &Server{
		logger:           logger,
		externalURL:      externalURL,
		tokenGenerator:   tokenGenerator,
		providerFactory:  providerFactory,
		basicAuthEnabled: basicAuthEnabled,
	}
}
