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
	URL string `long:"url" description:"CredHub server address used to access secrets."`

	PathPrefix string `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`

	TLS struct {
		CACerts    []string `long:"ca-cert"              description:"Paths to PEM-encoded CA cert files to use to verify the CredHub server SSL cert."`
		ClientCert string   `long:"client-cert"          description:"Path to the client certificate for mutual TLS authorization."`
		ClientKey  string   `long:"client-key"           description:"Path to the client private key for mutual TLS authorization."`
		Insecure   bool     `long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
	}

	UAA struct {
		ClientId     string `long:"client-id"     description:"Client ID for CredHub authorization."`
		ClientSecret string `long:"client-secret" description:"Client secret for CredHub authorization."`
	}
}

func (manager CredHubManager) IsConfigured() bool {
	return manager.URL != "" ||
		manager.UAA.ClientId != "" ||
		manager.UAA.ClientSecret != "" ||
		len(manager.TLS.CACerts) != 0 ||
		manager.TLS.ClientCert != "" ||
		manager.TLS.ClientKey != ""
}

func (manager CredHubManager) Validate() error {
	parsedUrl, err := url.Parse(manager.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", err)
	}

	// "foo" will parse without error (as a Path, with an empty Host)
	// so we'll do a few additional sanity checks that this is a valid URL
	if parsedUrl.Host == "" || !(parsedUrl.Scheme == "http" || parsedUrl.Scheme == "https") {
		return fmt.Errorf("invalid URL (must be http or https)")
	}

	return nil
}

func (manager CredHubManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	var options []credhub.Option

	if manager.TLS.Insecure {
		options = append(options, credhub.SkipTLSValidation(true))
	}

	caCerts := []string{}
	for _, cert := range manager.TLS.CACerts {
		contents, err := ioutil.ReadFile(cert)
		if err != nil {
			return nil, err
		}

		caCerts = append(caCerts, string(contents))
	}

	if len(caCerts) > 0 {
		options = append(options, credhub.CaCerts(caCerts...))
	}

	if manager.UAA.ClientId != "" && manager.UAA.ClientSecret != "" {
		options = append(options, credhub.Auth(auth.UaaClientCredentials(
			manager.UAA.ClientId,
			manager.UAA.ClientSecret,
		)))
	}

	if manager.TLS.ClientCert != "" && manager.TLS.ClientKey != "" {
		options = append(options, credhub.ClientCert(manager.TLS.ClientCert, manager.TLS.ClientKey))
	}

	ch, err := credhub.New(manager.URL, options...)
	if err != nil {
		return nil, err
	}

	return NewCredHubFactory(logger, ch, manager.PathPrefix), nil
}
