package credhub

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"sync"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/auth"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
)

type CredHubManager struct {
	URL string `long:"url" description:"CredHub server address used to access secrets."`

	PathPrefix string `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`

	TLS    TLS
	UAA    UAA
	Client *LazyCredhub
}

type TLS struct {
	CACerts    []string `long:"ca-cert"              description:"Paths to PEM-encoded CA cert files to use to verify the CredHub server SSL cert."`
	ClientCert string   `long:"client-cert"          description:"Path to the client certificate for mutual TLS authorization."`
	ClientKey  string   `long:"client-key"           description:"Path to the client private key for mutual TLS authorization."`
	Insecure   bool     `long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
}

type UAA struct {
	ClientId     string `long:"client-id"     description:"Client ID for CredHub authorization."`
	ClientSecret string `long:"client-secret" description:"Client secret for CredHub authorization."`
}

func (manager *CredHubManager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"url":           manager.URL,
		"path_prefix":   manager.PathPrefix,
		"ca_certs":      manager.TLS.CACerts,
		"uaa_client_id": manager.UAA.ClientId,
		"health":        health,
	}

	return json.Marshal(&response)
}

func (manager *CredHubManager) Init(log lager.Logger) error {
	var options []credhub.Option
	if manager.TLS.Insecure {
		options = append(options, credhub.SkipTLSValidation(true))
	}

	caCerts := []string{}
	for _, cert := range manager.TLS.CACerts {
		contents, err := ioutil.ReadFile(cert)
		if err != nil {
			return err
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

	manager.Client = newLazyCredhub(manager.URL, options)

	return nil
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

func (manager CredHubManager) Health() (*creds.HealthResponse, error) {
	healthResponse := &creds.HealthResponse{
		Method: "/health",
	}

	credhubObject, err := manager.Client.CredHub()
	if err != nil {
		return healthResponse, err
	}

	response, err := credhubObject.Client().Get(manager.URL + "/health")
	if err != nil {
		healthResponse.Error = err.Error()
		return healthResponse, nil
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		healthResponse.Error = "not ok"
		return healthResponse, nil
	}

	var credhubHealth struct {
		Status string `json:"status"`
	}

	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&credhubHealth)
	if err != nil {
		return nil, err
	}

	healthResponse.Response = credhubHealth

	return healthResponse, nil
}

func (manager CredHubManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	return NewCredHubFactory(logger, manager.Client, manager.PathPrefix), nil
}

type LazyCredhub struct {
	url     string
	options []credhub.Option
	credhub *credhub.CredHub
	mu      sync.Mutex
}

func newLazyCredhub(url string, options []credhub.Option) *LazyCredhub {
	return &LazyCredhub{
		url:     url,
		options: options,
	}
}

func (lc *LazyCredhub) CredHub() (*credhub.CredHub, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.credhub != nil {
		return lc.credhub, nil
	}

	ch, err := credhub.New(lc.url, lc.options...)
	if err != nil {
		return nil, err
	}

	lc.credhub = ch

	return lc.credhub, nil
}
