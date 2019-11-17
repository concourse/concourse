package creds

import (
	"fmt"
	"strings"
	"time"
)

type namedSecrets struct {
	secrets     Secrets
	name string
}

func NewNamedSecrets(secrets Secrets, name string) Secrets {
	return &namedSecrets{secrets: secrets, name: name}
}

// Get checks var_source if presents, then forward var to underlying secret manager.
// A `secretPath` with a var_source looks like "concourse/main/pipeline/myvault:foo",
// where "myvault" is the var_source name, and "concourse/main/pipeline/foo" is the
// real secretPath that should be forwarded to the underlying secret manager.
func (s namedSecrets) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	var sourceName, varName string

	parts := strings.Split(secretPath, ":")
	if len(parts) == 1 {
		varName = parts[0]
	} else if len(parts) == 2 {
		sourceName = parts[0]
		varName = parts[1]

		parts = strings.Split(sourceName, "/")
		sourceName = parts[len(parts)-1]

		parts = parts[:len(parts)-1]
		if len(parts) > 0 {
			varName = strings.Join(parts, "/") + "/" + varName
		}
	} else {
		return nil, nil, false, fmt.Errorf("invalid var: %s", secretPath)
	}

	if s.name != sourceName {
		return nil, nil, false, nil
	}

	return s.secrets.Get(varName)
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (s namedSecrets) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []SecretLookupPath {
	return s.secrets.NewSecretLookupPaths(teamName, pipelineName, allowRootPath)
}
