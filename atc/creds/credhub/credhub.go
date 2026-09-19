package credhub

import (
	"path"
	"time"

	"github.com/concourse/concourse/atc/creds"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/lager/v3"
)

var _ creds.Secrets = (*CredHubAtc)(nil)

type CredHubAtc struct {
	CredHub    *LazyCredhub
	logger     lager.Logger
	prefix     string
	sharedPath string
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (c CredHubAtc) NewSecretLookupPaths(params creds.SecretLookupParams, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	if len(params.Pipeline) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(c.prefix, params.Team, params.Pipeline)+"/"))
	}
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(c.prefix, params.Team)+"/"))
	if allowRootPath {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(c.prefix+"/"))
	}
	if c.sharedPath != "" {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(c.prefix, c.sharedPath)+"/"))
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (c CredHubAtc) Get(secretPath string, _ creds.SecretLookupParams) (any, *time.Time, bool, error) {
	var cred credentials.Credential
	var found bool
	var err error

	cred, found, err = c.findCred(secretPath)
	if err != nil {
		c.logger.Error("unable to retrieve credhub secret", err)
		return nil, nil, false, err
	}

	if !found {
		return nil, nil, false, nil
	}

	return cred.Value, nil, true, nil
}

func (c CredHubAtc) findCred(path string) (credentials.Credential, bool, error) {
	var cred credentials.Credential
	var err error

	ch, err := c.CredHub.CredHub()
	if err != nil {
		return cred, false, err
	}

	results, err := ch.FindByPartialName(path)
	if err != nil {
		return cred, false, err
	}

	// same as https://github.com/cloudfoundry/credhub-cli/blob/main/commands/find.go#L22
	if len(results.Credentials) == 0 {
		return cred, false, nil
	}

	cred, err = ch.GetLatestVersion(path)
	if err != nil {
		return cred, false, err
	}

	return cred, true, nil
}
