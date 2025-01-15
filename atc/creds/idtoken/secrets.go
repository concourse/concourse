package idtoken

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"
)

type Secrets struct {
	TokenGenerator *TokenGenerator
}

type fixedSecretPath struct {
	fixedPath string
}

func (p fixedSecretPath) VariableToSecretPath(_ string) (string, error) {
	return p.fixedPath, nil
}

func (secrets *Secrets) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	// there is no real "loolupPath" involved
	// just return something from which team and pipeline can be extracted later
	return []creds.SecretLookupPath{fixedSecretPath{path.Join(teamName, pipelineName)}}
}

func (secrets *Secrets) Get(secretPath string) (interface{}, *time.Time, bool, error) {
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
