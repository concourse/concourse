package idtoken

import (
	"fmt"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"
)

var _ creds.Secrets = (*IDToken)(nil)

type IDToken struct {
	TokenGenerator *TokenGenerator
}

func (secrets *IDToken) NewSecretLookupPaths(params creds.SecretLookupParams, _ bool) []creds.SecretLookupPath {
	// Always use the Job scope to generate the lookup path to avoid cache
	// collisions between jobs within and across pipelines/teams
	return []creds.SecretLookupPath{creds.NewSecretLookupWithPrefix(generateSubject(SubjectScopeJob, params) + "/")}
}

func (secrets *IDToken) Get(secretPath string, params creds.SecretLookupParams) (any, *time.Time, bool, error) {
	secretPath, found := strings.CutPrefix(secretPath, generateSubject(SubjectScopeJob, params)+"/")
	if !found {
		return nil, nil, false, fmt.Errorf("idtoken credential provider was called with different secret params")
	}

	if secretPath != "token" {
		return nil, nil, false, fmt.Errorf("idtoken credential provider only supports the field 'token'")
	}

	if params.IsEmpty() {
		return nil, nil, false, fmt.Errorf("idtoken credential provider was called with empty params")
	}

	token, validUntil, err := secrets.TokenGenerator.GenerateToken(params)
	if err != nil {
		return nil, nil, false, err
	}

	return token, &validUntil, true, nil
}
