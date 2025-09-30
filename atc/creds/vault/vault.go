package vault

import (
	"path"
	"time"

	"github.com/concourse/concourse/atc/creds"

	vaultapi "github.com/hashicorp/vault/api"
)

type VaultLoginTimeout struct{}

func (e VaultLoginTimeout) Error() string {
	return "timed out to login to vault"
}

// A SecretReader reads a vault secret from the given path. It should
// be thread safe!
type SecretReader interface {
	Read(path string) (*vaultapi.Secret, error)
}

// Vault converts a vault secret to our completely untyped secret
// data.
type Vault struct {
	SecretReader    SecretReader
	Prefix          string
	Prefixes        []string
	LookupTemplates []*creds.SecretTemplate
	SharedPath      string
	LoggedIn        <-chan struct{}
	LoginTimeout    time.Duration
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (v Vault) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, tmpl := range v.LookupTemplates {
		if lPath := creds.NewSecretLookupWithTemplate(tmpl, teamName, pipelineName); lPath != nil {
			lookupPaths = append(lookupPaths, lPath)
		}
	}
	for _, prefix := range getPrefixes(v.Prefixes, v.Prefix) {
		if v.SharedPath != "" {
			lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(prefix, v.SharedPath)+"/"))
		}
		if allowRootPath {
			lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(prefix+"/"))
		}
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (v Vault) Get(secretPath string) (any, *time.Time, bool, error) {
	if v.LoggedIn != nil {
		select {
		case <-v.LoggedIn:
		case <-time.After(v.LoginTimeout):
			return nil, nil, false, VaultLoginTimeout{}
		}
	}

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
		if secret.LeaseDuration != -1 {
			// The lease duration is TTL: the time in seconds for which the lease is valid
			// A consumer of this secret must renew the lease within that time.
			duration := time.Duration(secret.LeaseDuration) * time.Second / 2
			expiration := time.Now().Add(duration)
			return secret, &expiration, true, nil
		}

		return secret, nil, true, nil
	}

	return nil, nil, false, nil
}
