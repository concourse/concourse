package creds

import (
	"time"

	"github.com/concourse/concourse/atc"
)

//counterfeiter:generate . SecretsFactory
type SecretsFactory interface {
	// NewSecrets returns an instance of a secret manager, capable of retrieving individual secrets
	NewSecrets() Secrets
}

//counterfeiter:generate . Secrets
type Secrets interface {
	// Every credential manager needs to be able to return (secret, secret_expiration_time, exists, error) based on the secret path
	Get(string) (any, *time.Time, bool, error)

	// NewSecretLookupPaths returns an instance of lookup policy, which can transform pipeline ((var)) into one or more secret paths, based on team name and pipeline name
	NewSecretLookupPaths(string, string, bool) []SecretLookupPath
}

type SecretLookupContext struct {
	Team         string
	Pipeline     string
	InstanceVars atc.InstanceVars
	Job          string
}

func (s SecretLookupContext) IsEmpty() bool {
	return s.Team == "" && s.Pipeline == "" && s.InstanceVars == nil && s.Job == ""
}

// SecretsWithContext is an extended version of the Secrets interface that allows callers to pass in additional information
//
//counterfeiter:generate . SecretsWithContext
type SecretsWithContext interface {
	Secrets
	GetWithContext(path string, context SecretLookupContext) (any, *time.Time, bool, error)
	NewSecretLookupPathsWithContext(context SecretLookupContext, allowRootPath bool) []SecretLookupPath
}

// if the provided secrets implements SecretsWithContext, it calls GetWithContext on it with the provided context, otherwise Get is called
func getWithContext(secrets Secrets, path string, context SecretLookupContext) (any, *time.Time, bool, error) {
	if contextAwareSecret, isContextAware := secrets.(SecretsWithContext); isContextAware {
		return contextAwareSecret.GetWithContext(path, context)
	}
	return secrets.Get(path)
}

// if the provided secrets implements SecretsWithContext, it calls NewSecretLookupPathsWithContext on it with the provided context, otherwise NewSecretLookupPaths is called
func newSecretLookupPathsWithContext(secrets Secrets, context SecretLookupContext, allowRoot bool) []SecretLookupPath {
	if contextAwareSecret, isContextAware := secrets.(SecretsWithContext); isContextAware {
		return contextAwareSecret.NewSecretLookupPathsWithContext(context, allowRoot)
	}
	return secrets.NewSecretLookupPaths(context.Team, context.Pipeline, allowRoot)
}
