package localfile

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
)

type SecretsFactory struct {
	path   string
	logger lager.Logger
}

func (factory *SecretsFactory) NewSecrets() creds.Secrets {
	return &Secrets{
		path:   factory.path,
		logger: factory.logger,
	}
}
