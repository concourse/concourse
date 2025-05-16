package idtoken

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"
)

type IDToken struct {
	TokenGenerator *TokenGenerator
}

type fixedSecretPath struct {
	fixedPath string
}

func (p fixedSecretPath) VariableToSecretPath(secretName string) (string, error) {
	if secretName != "token" {
		return "", fmt.Errorf("idtoken credential provider only supports the field 'token'")
	}
	return p.fixedPath, nil
}

func (secrets *IDToken) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	// there is no real "lookupPath" involved
	// just return something from which team and pipeline can be extracted later
	return []creds.SecretLookupPath{fixedSecretPath{path.Join(teamName, pipelineName)}}
}

func (secrets *IDToken) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	parts := strings.Split(secretPath, "/")
	if len(parts) != 2 {
		return nil, nil, false, fmt.Errorf("secretPath should have exactly 2 parts")
	}
	team := parts[0]
	pipeline := parts[1]

	token, _, err := secrets.TokenGenerator.GenerateToken(team, pipeline)
	if err != nil {
		return nil, nil, false, err
	}

	return token, nil, true, nil
}
