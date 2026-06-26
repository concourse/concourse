package credhub

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds"
)

type credhubFactory struct {
	credhub    *LazyCredhub
	logger     lager.Logger
	prefix     string
	sharedPath string
}

func NewCredHubFactory(logger lager.Logger, credhub *LazyCredhub, prefix string, sharedPath string) *credhubFactory {
	return &credhubFactory{
		credhub:    credhub,
		logger:     logger,
		prefix:     prefix,
		sharedPath: sharedPath,
	}
}

func (factory *credhubFactory) NewSecrets() creds.Secrets {
	return &CredHubAtc{
		CredHub:    factory.credhub,
		logger:     factory.logger,
		prefix:     factory.prefix,
		sharedPath: factory.sharedPath,
	}
}
