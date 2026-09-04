package noop

import (
	"time"

	"github.com/concourse/concourse/v8/atc/creds"
)

type Noop struct{}

func (n Noop) NewSecretLookupPaths(string, string, bool) []creds.SecretLookupPath {
	return []creds.SecretLookupPath{}
}

func (n Noop) Get(secretPath string) (any, *time.Time, bool, error) {
	return nil, nil, false, nil
}
