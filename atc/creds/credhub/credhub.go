package credhub

import (
	"path"
	"time"

	"github.com/concourse/concourse/atc/creds"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/lager/v3"
)

type CredHubAtc struct {
	CredHub  *LazyCredhub
	logger   lager.Logger
	prefix   string
	prefixes []string
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (c CredHubAtc) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	prefixes := getPrefixes(c.prefixes, c.prefix)
	if len(pipelineName) > 0 {
		for _, prefix := range prefixes {
			lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(prefix, teamName, pipelineName)+"/"))
		}
	}
	for _, prefix := range prefixes {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(prefix, teamName)+"/"))
	}
	if allowRootPath {
		for _, prefix := range prefixes {
			lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(prefix+"/"))
		}
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (c CredHubAtc) Get(secretPath string) (any, *time.Time, bool, error) {
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

func getPrefixes(prefixes []string, prefix string) []string {
	if prefix != "" {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}
