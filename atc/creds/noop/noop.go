package noop

import (
	"github.com/concourse/concourse/v5/atc/creds"
	"time"
)

type Noop struct{}

func (n Noop) NewSecretLookupPaths(string, string) []creds.SecretLookupPath {
	return []creds.SecretLookupPath{}
}

func (n Noop) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	return nil, nil, false, nil
}
