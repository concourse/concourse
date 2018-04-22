package credhub

import (
	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/concourse/atc/creds"
)

type credhubFactory struct {
	credhub lazyCredhub
	logger  lager.Logger
	prefix  string
}

func NewCredHubFactory(logger lager.Logger, credhub lazyCredhub, prefix string) *credhubFactory {
	return &credhubFactory{
		credhub: lazyCredhub,
		logger:  logger,
		prefix:  prefix,
	}
}

func (factory *credhubFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return &CredHubAtc{
		CredHub:      factory.credhub,
		PathPrefix:   factory.prefix,
		TeamName:     teamName,
		logger:       factory.logger,
		PipelineName: pipelineName,
	}
}
