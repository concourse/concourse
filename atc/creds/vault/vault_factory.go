package vault

import (
	"time"

	"github.com/concourse/concourse/atc/creds"
)

// The vaultFactory will return a vault implementation of vars.Variables.
type vaultFactory struct {
	sr              SecretReader
	prefix          string
	sharedPath      string
	lookupTemplates []*creds.SecretTemplate
	loggedIn        <-chan struct{}
	loginTimeout    time.Duration
}

func NewVaultFactory(sr SecretReader, loginTimeout time.Duration, loggedIn <-chan struct{}, prefix string, lookupTemplates []*creds.SecretTemplate, sharedPath string) *vaultFactory {
	factory := &vaultFactory{
		sr:              sr,
		prefix:          prefix,
		lookupTemplates: lookupTemplates,
		sharedPath:      sharedPath,
		loggedIn:        loggedIn,
		loginTimeout:    loginTimeout,
	}

	return factory
}

// NewSecrets will block until the loggedIn channel passed to the constructor signals a successful login.
func (factory *vaultFactory) NewSecrets() creds.Secrets {
	select {
	case <-factory.loggedIn:
	case <-time.After(factory.loginTimeout):
	}

	return &Vault{
		SecretReader:    factory.sr,
		Prefix:          factory.prefix,
		LookupTemplates: factory.lookupTemplates,
		SharedPath:      factory.sharedPath,
	}
}
