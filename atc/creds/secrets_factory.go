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
	// Every credential manager needs to be able to return
	// (secret, secret_expiration_time, exists, error) based on the secret path
	Get(path string, params SecretLookupParams) (any, *time.Time, bool, error)

	// NewSecretLookupPaths returns an instance of lookup policy, which can
	// transform pipeline ((var)) into one or more secret paths, based on
	// SecretLookupParams
	NewSecretLookupPaths(params SecretLookupParams, allowRootPath bool) []SecretLookupPath
}

type SecretLookupParams struct {
	Team         string
	Pipeline     string
	InstanceVars atc.InstanceVars
	Job          string
}

func (s SecretLookupParams) IsEmpty() bool {
	return s.Team == "" && s.Pipeline == "" && len(s.InstanceVars) == 0 && s.Job == ""
}
