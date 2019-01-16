package keyvault

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
)

type keyVaultFactory struct {
	log        lager.Logger
	pathPrefix string
	reader     SecretReader
}

// NewKeyVaultFactory returns an Azure Key Vault implementation of the
// creds.VariablesFactory interface
func NewKeyVaultFactory(log lager.Logger, reader SecretReader, prefix string) creds.VariablesFactory {
	return &keyVaultFactory{
		log:        log,
		pathPrefix: prefix,
		reader:     reader,
	}
}

// NewVariables implements the VariablesFactory interface and returns a
// Variables implementation for Azure Key Vault
func (factory *keyVaultFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return NewKeyVault(factory.log, factory.reader, factory.pathPrefix, teamName, pipelineName)
}
