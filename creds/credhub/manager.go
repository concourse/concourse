package credhub

import (
	"fmt"
	"io/ioutil"
	"net/url"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/concourse/atc/creds"
)

type CredHubManager struct {
	URL          string   `long:"url" description:"CredHub server address used to access secrets."`
	PathPrefix   string   `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`
	CACerts      []string `long:"ca-cert" description:"Paths to PEM-encoded CA cert files to use to verify the CredHub server SSL cert."`
	Insecure     bool     `long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
	ClientId     string   `long:"client-id" description:"Client ID for CredHub authorization."`
	ClientSecret string   `long:"client-secret" description:"Client secret for CredHub authorization."`
	caCerts      []string `no-flag:true`
}

func (manager CredHubManager) IsConfigured() bool {
	return manager.URL != "" && manager.ClientId != "" && manager.ClientSecret != ""
}

func (manager CredHubManager) Validate() error {
	parsedUrl, err := url.Parse(manager.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", err)
	}

	if parsedUrl.Scheme == "https" {
		if len(manager.CACerts) < 1 && !manager.Insecure {
			return fmt.Errorf("CACerts or insecure needs to be set for secure urls")
		}
	}

	if len(manager.CACerts) > 1 {
		for _, cert := range manager.CACerts {
			contents, err := ioutil.ReadFile(cert)
			if err != nil {
				return fmt.Errorf("Could not read CaCert at path %s", cert)
			}
			manager.caCerts = append(manager.caCerts, string(contents))
		}
	}

	return nil
}

func (manager CredHubManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	var skipTls credhub.Option

	if manager.Insecure {
		skipTls = credhub.SkipTLSValidation()
	}

	ch, err := credhub.New(manager.URL,
		skipTls,
		credhub.Auth(auth.UaaClientCredentials(manager.ClientId, manager.ClientSecret)))

	return NewCredHubFactory(logger, ch, manager.PathPrefix), err
}
