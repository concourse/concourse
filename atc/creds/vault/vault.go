package vault

import (
	"path"
	"time"

	"github.com/concourse/concourse/atc/creds"

	vaultapi "github.com/hashicorp/vault/api"
)

// A SecretReader reads a vault secret from the given path. It should
// be thread safe!
type SecretReader interface {
	Read(path string) (*vaultapi.Secret, error)
}

// Vault converts a vault secret to our completely untyped secret
// data.
type Vault struct {
	SecretReader SecretReader
	Prefix       string
	SharedPath   string
	SkipTeamPath bool
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (v Vault) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	if !v.SkipTeamPath {
		if len(pipelineName) > 0 {
			lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(v.Prefix, teamName, pipelineName)+"/"))
		}
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(v.Prefix, teamName)+"/"))
	}
	if v.SharedPath != "" {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(v.Prefix, v.SharedPath)+"/"))
	}
	if allowRootPath {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(v.Prefix+"/"))
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (v Vault) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	secret, expiration, found, err := v.findSecret(secretPath)
	if err != nil {
		return nil, nil, false, err
	}
	if !found {
		return nil, nil, false, nil
	}

	val, found := secret.Data["value"]
	if found {
		return val, expiration, true, nil
	}

	return secret.Data, expiration, true, nil
}

func (v Vault) findSecret(path string) (*vaultapi.Secret, *time.Time, bool, error) {
	secret, err := v.SecretReader.Read(path)
	if err != nil {
		return nil, nil, false, err
	}

	if secret != nil {
		// The lease duration is TTL: the time in seconds for which the lease is valid
		// A consumer of this secret must renew the lease within that time.
		duration := time.Duration(secret.LeaseDuration) * time.Second / 2
		expiration := time.Now().Add(duration)
		return secret, &expiration, true, nil
	}

	return nil, nil, false, nil
}
