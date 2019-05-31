package credhub

import (
	"github.com/concourse/concourse/v5/atc/creds"
	"path"
	"time"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/lager"
)

type CredHubAtc struct {
	CredHub *LazyCredhub
	logger  lager.Logger
	prefix  string
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (c CredHubAtc) NewSecretLookupPaths(teamName string, pipelineName string) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	if len(pipelineName) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(c.prefix, teamName, pipelineName)+"/"))
	}
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(c.prefix, teamName)+"/"))
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (c CredHubAtc) Get(secretPath string) (interface{}, *time.Time, bool, error) {
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

	var result = cred.Value

	if standardMap, ok := cred.Value.(map[string]interface{}); ok {
		// TODO - we should do this recursively since the cpp4life go-path library
		// does not support map[string]interface{} types when looking for
		// nested values
		evenLessTyped := map[interface{}]interface{}{}

		for k, v := range standardMap {
			evenLessTyped[k] = v
		}

		result = evenLessTyped
	}

	return result, nil, true, nil
}

func (c CredHubAtc) findCred(path string) (credentials.Credential, bool, error) {
	var cred credentials.Credential
	var err error

	ch, err := c.CredHub.CredHub()
	if err != nil {
		return cred, false, err
	}

	_, err = ch.FindByPath(path)
	if err != nil {
		return cred, false, err
	}

	cred, err = ch.GetLatestVersion(path)
	if _, ok := err.(*credhub.Error); ok {
		return cred, false, nil
	}

	if err != nil {
		return cred, false, err
	}

	return cred, true, nil
}
