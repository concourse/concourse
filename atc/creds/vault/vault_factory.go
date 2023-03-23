package vault

import (
	"time"

	"github.com/concourse/concourse/atc/creds"
)

// The vaultFactory will return a vault implementation of vars.Variables.
type vaultFactory struct {
	sr              SecretReader
	prefix          string
	prefixes        []string
	sharedPath      string
	lookupTemplates []*creds.SecretTemplate
	loggedIn        <-chan struct{}
	loginTimeout    time.Duration
}

func NewVaultFactory(sr SecretReader, loginTimeout time.Duration, loggedIn <-chan struct{}, prefix string, prefixes []string, lookupTemplates []*creds.SecretTemplate, sharedPath string) *vaultFactory {
	factory := &vaultFactory{
		sr:              sr,
		prefix:          prefix,
		prefixes:        prefixes,
		lookupTemplates: lookupTemplates,
		sharedPath:      sharedPath,
		loggedIn:        loggedIn,
		loginTimeout:    loginTimeout,
	}

	return factory
}

func (factory *vaultFactory) NewSecrets() creds.Secrets {
	return &Vault{
		SecretReader:    factory.sr,
		Prefix:          factory.prefix,
		Prefixes:        factory.prefixes,
		LookupTemplates: factory.lookupTemplates,
		SharedPath:      factory.sharedPath,
		LoginTimeout:    factory.loginTimeout,
		LoggedIn:        factory.loggedIn,
	}
}
