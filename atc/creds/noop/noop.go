package noop

import (
	"time"

	"github.com/concourse/concourse/atc/creds"
)

var _ creds.Secrets = (*Noop)(nil)

type Noop struct{}

func (n Noop) NewSecretLookupPaths(creds.SecretLookupParams, bool) []creds.SecretLookupPath {
	return []creds.SecretLookupPath{}
}

func (n Noop) Get(secretPath string, _ creds.SecretLookupParams) (any, *time.Time, bool, error) {
	return nil, nil, false, nil
}
