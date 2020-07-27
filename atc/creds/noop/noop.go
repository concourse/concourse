package noop

import (
	"time"

	"github.com/concourse/concourse/atc/creds"
)

type Noop struct{}

func (n Noop) NewSecretLookupPaths(string, string, bool) []creds.SecretLookupPath {
	return []creds.SecretLookupPath{}
}

func (n Noop) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	return nil, nil, false, nil
}
