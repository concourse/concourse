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
	"github.com/concourse/concourse/atc/creds"
)

const managerName = "credhub"

type CredHubManager struct {
	URL string `yaml:"url,omitempty"`

	PathPrefix string `yaml:"path_prefix,omitempty"`

	TLS    TLS          `yaml:",inline"`
	UAA    UAA          `yaml:",inline"`
	Client *LazyCredhub `yaml:",omitempty"`
}

type TLS struct {
	CACerts    []string `yaml:"ca_cert,omitempty"`
	ClientCert string   `yaml:"client_cert,omitempty"`
	ClientKey  string   `yaml:"client_key,omitempty"`
	Insecure   bool     `yaml:"insecure_skip_verify,omitempty"`
}

type UAA struct {
	ClientId     string `yaml:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty"`
}

func (manager *CredHubManager) Name() string {
	return managerName
}

func (manager *CredHubManager) Config() interface{} {
	return manager
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

func (manager CredHubManager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
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

func (manager CredHubManager) Close(logger lager.Logger) {
	// TODO - to implement
}
