package idtoken

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc/creds"
)

type IDToken struct {
	TokenGenerator *TokenGenerator
}

func (secrets *IDToken) NewSecretLookupPathsWithContext(context creds.SecretLookupContext, allowRootPath bool) []creds.SecretLookupPath {
	// returning no paths will result in GetWithContext() being called directly with the secret-name
	return []creds.SecretLookupPath{}
}

func (secrets *IDToken) GetWithContext(secretPath string, context creds.SecretLookupContext) (interface{}, *time.Time, bool, error) {
	if secretPath != "token" {
		return nil, nil, false, fmt.Errorf("idtoken credential provider only supports the field 'token'")
	}

	if context.IsEmpty() {
		return nil, nil, false, fmt.Errorf("idtoken credential provider was called with empty context")
	}

	token, _, err := secrets.TokenGenerator.GenerateToken(context)
	if err != nil {
		return nil, nil, false, err
	}

	return token, nil, true, nil
}

func (secrets *IDToken) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	return nil
}

func (secrets *IDToken) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	return nil, nil, false, fmt.Errorf("IDToken provider can only be used with context")
}
