package credhub

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
)

type credhubFactory struct {
	credhub *LazyCredhub
	logger  lager.Logger
	prefix  string
}

func NewCredHubFactory(logger lager.Logger, credhub *LazyCredhub, prefix string) *credhubFactory {
	return &credhubFactory{
		credhub: credhub,
		logger:  logger,
		prefix:  prefix,
	}
}

func (factory *credhubFactory) NewSecrets() creds.Secrets {
	return &CredHubAtc{
		CredHub: factory.credhub,
		logger:  factory.logger,
		prefix:  factory.prefix,
	}
}
