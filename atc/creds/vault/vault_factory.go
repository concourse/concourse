package vault

import (
	"github.com/concourse/concourse/atc/creds"
)

// The vaultFactory will return a vault implementation of creds.Variables.
type vaultFactory struct {
	sr       SecretReader
	prefix   string
	loggedIn <-chan struct{}
}

func NewVaultFactory(sr SecretReader, loggedIn <-chan struct{}, prefix string) *vaultFactory {
	factory := &vaultFactory{
		sr:       sr,
		prefix:   prefix,
		loggedIn: loggedIn,
	}

	return factory
}

func (factory *vaultFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return &Vault{
		LoggedIn:     factory.loggedIn,
		SecretReader: factory.sr,
		PathPrefix:   factory.prefix,
		TeamName:     teamName,
		PipelineName: pipelineName,
	}
}
