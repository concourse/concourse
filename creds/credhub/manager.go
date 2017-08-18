package credhub

import (
	"fmt"
	"io/ioutil"
	"net/url"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth/uaa"
	"github.com/concourse/atc/creds"
)

type CredhubManager struct {
	// FIXME Update descriptions
	URL          string   `long:"url" description:"Credhub server address used to access secrets."`
	PathPrefix   string   `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`
	CaCerts      []string `long:"ca-certs" description:"Paths to PEM-encoded CA cert files to use to verify the credhub server SSL cert."`
	Insecure     bool     `long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
	ClientId     string   `long:"client-id" description:"Client ID for Credhub authorization."`
	ClientSecret string   `long:"client-secret" description:"Client secret for Credhub authorization."`
	caCerts      []string `no-flag:true`
}

func (manager CredhubManager) IsConfigured() bool {
	return manager.URL != "" && manager.ClientId != "" && manager.ClientSecret != ""
}

func (manager CredhubManager) Validate() error {
	parsedUrl, err := url.Parse(manager.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", err)
	}

	if parsedUrl.Scheme == "https" {
		if len(manager.CaCerts) < 1 && !manager.Insecure {
			return fmt.Errorf("CaCerts or insecure needs to be set for secure urls")
		}
	}

	if len(manager.CaCerts) > 1 {
		for _, cert := range manager.CaCerts {
			contents, err := ioutil.ReadFile(cert)
			if err != nil {
				return fmt.Errorf("Could not read CaCert at path %s", cert)
			}
			manager.caCerts = append(manager.caCerts, string(contents))
		}
	}

	return nil
}

func (manager CredhubManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	ch, err := credhub.New(manager.URL,
		credhub.SkipTLSValidation(),
		credhub.AuthBuilder(uaa.ClientCredentialsGrantBuilder(manager.ClientId, manager.ClientSecret)))

	return NewCredhubFactory(logger, ch, manager.PathPrefix), err
}
