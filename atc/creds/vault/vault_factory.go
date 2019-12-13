package vault

import (
	"time"

	"github.com/concourse/concourse/atc/creds"
)

// The vaultFactory will return a vault implementation of vars.Variables.
type vaultFactory struct {
	sr           SecretReader
	prefix       string
	sharedPath   string
	skipTeamPath bool
	loggedIn     <-chan struct{}
}

func NewVaultFactory(sr SecretReader, loggedIn <-chan struct{}, prefix string, sharedPath string, skipTeamPath bool) *vaultFactory {
	factory := &vaultFactory{
		sr:           sr,
		prefix:       prefix,
		sharedPath:   sharedPath,
		skipTeamPath: skipTeamPath,
		loggedIn:     loggedIn,
	}

	return factory
}

// NewSecrets will block until the loggedIn channel passed to the constructor signals a successful login.
func (factory *vaultFactory) NewSecrets() creds.Secrets {
	select {
	case <-factory.loggedIn:
	case <-time.After(5 * time.Second):
	}

	return &Vault{
		SecretReader: factory.sr,
		Prefix:       factory.prefix,
		SharedPath:   factory.sharedPath,
		SkipTeamPath: factory.skipTeamPath,
	}
}
