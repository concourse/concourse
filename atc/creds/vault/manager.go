package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"time"

	"code.cloudfoundry.org/lager"
	"gopkg.in/yaml.v2"

	"github.com/concourse/concourse/atc/creds"
)

const managerName = "vault"

type VaultManager struct {
	URL string `yaml:"url"`

	PathPrefix      string        `yaml:"path_prefix,omitempty"`
	LookupTemplates []string      `yaml:"lookup_templates,omitempty"`
	SharedPath      string        `yaml:"shared_path,omitempty"`
	Namespace       string        `yaml:"namespace,omitempty"`
	LoginTimeout    time.Duration `yaml:"login_timeout,omitempty"`
	QueryTimeout    time.Duration `yaml:"query_timeout,omitempty"`

	TLS  TLSConfig
	Auth AuthConfig

	Client        *APIClient
	ReAuther      *ReAuther
	SecretFactory *vaultFactory
}

type TLSConfig struct {
	CACert     string `yaml:"ca_cert,omitempty"`
	CACertFile string `yaml:"ca_cert_file,omitempty" env:"CONCOURSE_VAULT_CA_CERT_FILE,CONCOURSE_VAULT_CA_CERT"`
	CAPath     string `yaml:"ca_path,omitempty"`

	ClientCert     string `yaml:"client_cert,omitempty"`
	ClientCertFile string `yaml:"client_cert_file,omitempty" env:"CONCOURSE_VAULT_CLIENT_CERT_FILE,CONCOURSE_VAULT_CLIENT_CERT"`

	ClientKey     string `yaml:"client_key,omitempty"`
	ClientKeyFile string `yaml:"client_key_file,omitempty" env:"CONCOURSE_VAULT_CLIENT_KEY_FILE,CONCOURSE_VAULT_CLIENT_KEY"`

	ServerName string `yaml:"server_name,omitempty"`
	Insecure   bool   `yaml:"insecure_skip_verify,omitempty"`
}

type AuthConfig struct {
	ClientToken string `yaml:"client_token,omitempty"`

	Backend       string        `yaml:"auth_backend,omitempty"`
	BackendMaxTTL time.Duration `yaml:"auth_backend_max_ttl,omitempty"`
	RetryMax      time.Duration `yaml:"auth_retry_max,omitempty"`
	RetryInitial  time.Duration `yaml:"auth_retry_initial,omitempty"`

	Params map[string]string `yaml:"auth_params,omitempty"`
}

func (manager *VaultManager) Name() string {
	return managerName
}

func (manager *VaultManager) Config() interface{} {
	return manager
}

func (manager *VaultManager) Init(log lager.Logger) error {
	var err error

	manager.Client, err = NewAPIClient(log, manager.URL, manager.TLS, manager.Auth, manager.Namespace, manager.QueryTimeout)
	if err != nil {
		return err
	}

	return nil
}

func (manager *VaultManager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]interface{}{
		"url":                manager.URL,
		"path_prefix":        manager.PathPrefix,
		"lookup_templates":   manager.LookupTemplates,
		"shared_path":        manager.SharedPath,
		"namespace":          manager.Namespace,
		"ca_cert":            manager.TLS.CACert,
		"server_name":        manager.TLS.ServerName,
		"auth_backend":       manager.Auth.Backend,
		"auth_max_ttl":       manager.Auth.BackendMaxTTL,
		"auth_retry_max":     manager.Auth.RetryMax,
		"auth_retry_initial": manager.Auth.RetryInitial,
		"health":             health,
	})
}

func (manager *VaultManager) ApplyConfig(config interface{}) error {
	// apply defaults
	manager.PathPrefix = "/concourse"
	manager.Auth.RetryMax = 5 * time.Minute
	manager.Auth.RetryInitial = time.Second
	manager.LoginTimeout = 60 * time.Second
	manager.QueryTimeout = 60 * time.Second

	configBytes, ok := config.([]byte)
	if !ok {
		return fmt.Errorf("invalid config: %s", config)
	}

	err := yaml.Unmarshal(configBytes, &manager)
	if err != nil {
		return err
	}

	// Fill in default templates if not otherwise set (done here so
	// that these are effective all together or not at all, rather
	// than combining the defaults with a user's custom setting)
	if manager.LookupTemplates == nil {
		manager.LookupTemplates = []string{
			"/{{.Team}}/{{.Pipeline}}/{{.Secret}}",
			"/{{.Team}}/{{.Secret}}",
		}
	}

	return nil
}

func (manager VaultManager) IsConfigured() bool {
	return manager.URL != ""
}

func (manager VaultManager) Validate() error {
	_, err := url.Parse(manager.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", err)
	}

	if manager.PathPrefix == "" {
		return fmt.Errorf("path prefix must be a non-empty string")
	}

	for i, tmpl := range manager.LookupTemplates {
		name := fmt.Sprintf("lookup-template-%d", i)
		if _, err := creds.BuildSecretTemplate(name, manager.PathPrefix+tmpl); err != nil {
			return err
		}
	}

	if manager.Auth.ClientToken != "" {
		return nil
	}

	if manager.Auth.Backend != "" {
		return nil
	}

	return errors.New("must configure client token or auth backend")
}

func (manager VaultManager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "/v1/sys/health",
	}

	response, err := manager.Client.health()
	if err != nil {
		health.Error = err.Error()
		return health, nil
	}

	health.Response = response
	return health, nil
}

func (manager *VaultManager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
	if manager.SecretFactory == nil {

		templates := []*creds.SecretTemplate{}
		for i, tmpl := range manager.LookupTemplates {
			name := fmt.Sprintf("lookup-template-%d", i)
			scopedTemplate := path.Join(manager.PathPrefix, tmpl)
			if template, err := creds.BuildSecretTemplate(name, scopedTemplate); err != nil {
				return nil, err
			} else {
				templates = append(templates, template)
			}
		}

		manager.ReAuther = NewReAuther(
			logger,
			manager.Client,
			manager.Auth.BackendMaxTTL,
			manager.Auth.RetryInitial,
			manager.Auth.RetryMax,
		)

		manager.SecretFactory = NewVaultFactory(
			manager.Client,
			manager.LoginTimeout,
			manager.ReAuther.LoggedIn(),
			manager.PathPrefix,
			templates,
			manager.SharedPath,
		)
	}

	return manager.SecretFactory, nil
}

func (manager VaultManager) Close(logger lager.Logger) {
	manager.ReAuther.Close()
}
