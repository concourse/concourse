package vault

import (
	"time"

	"github.com/concourse/concourse/atc/creds"
)

// The vaultFactory will return a vault implementation of creds.Variables.
type vaultFactory struct {
	sr       SecretReader
	prefix   string
	sharedPath string
	loggedIn <-chan struct{}
}

func NewVaultFactory(sr SecretReader, loggedIn <-chan struct{}, prefix string, sharedPath string) *vaultFactory {
	factory := &vaultFactory{
		sr:       sr,
		prefix:   prefix,
		sharedPath: sharedPath,
		loggedIn: loggedIn,
	}

	return factory
}

// NewVariables will block until the loggedIn channel passed to the
// constructor signals a successful login.
func (factory *vaultFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	select {
	case <-factory.loggedIn:
	case <-time.After(5 * time.Second):
	}

	return &Vault{
		SecretReader: factory.sr,
		PathPrefix:   factory.prefix,
		SharedPath:   factory.sharedPath,
		TeamName:     teamName,
		PipelineName: pipelineName,
	}
}
