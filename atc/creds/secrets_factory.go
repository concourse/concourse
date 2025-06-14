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

type SecretLookupParams struct {
	Team         string
	Pipeline     string
	InstanceVars atc.InstanceVars
	Job          string
}

func (s SecretLookupParams) IsEmpty() bool {
	return s.Team == "" && s.Pipeline == "" && s.InstanceVars == nil && s.Job == ""
}

// SecretsWithParams is an extended version of the Secrets interface that allows callers to pass in additional information
//
//counterfeiter:generate . SecretsWithParams
type SecretsWithParams interface {
	Secrets
	GetWithParams(path string, params SecretLookupParams) (any, *time.Time, bool, error)
	NewSecretLookupPathsWithParams(params SecretLookupParams, allowRootPath bool) []SecretLookupPath
}

// if the provided secrets implements SecretsWithParams, it calls GetWithParams on it with the provided params, otherwise Get is called
func GetWithParams(secrets Secrets, path string, params SecretLookupParams) (any, *time.Time, bool, error) {
	if paramAwareSecret, isParamAware := secrets.(SecretsWithParams); isParamAware {
		return paramAwareSecret.GetWithParams(path, params)
	}
	return secrets.Get(path)
}

// if the provided secrets implements SecretsWithParams, it calls NewSecretLookupPathsWithParams on it with the provided params, otherwise NewSecretLookupPaths is called
func NewSecretLookupPathsWithParams(secrets Secrets, params SecretLookupParams, allowRoot bool) []SecretLookupPath {
	if paramAwareSecret, isParamAware := secrets.(SecretsWithParams); isParamAware {
		return paramAwareSecret.NewSecretLookupPathsWithParams(params, allowRoot)
	}
	return secrets.NewSecretLookupPaths(params.Team, params.Pipeline, allowRoot)
}
