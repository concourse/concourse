package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
	vaultapi "github.com/hashicorp/vault/api"
)

type VaultManager struct {
	URL string `long:"url" description:"Vault server address used to access secrets."`

	PathPrefix string `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`
	SharedPath string `long:"shared-path" description:"Path under which to lookup shared credentials."`

	TLS  TLS
	Auth AuthConfig

	Client        *APIClient
	ReAuther      *ReAuther
	SecretFactory *vaultFactory
}

type TLS struct {
	CACert     string `long:"ca-cert"              description:"Path to a PEM-encoded CA cert file to use to verify the vault server SSL cert."`
	CAPath     string `long:"ca-path"              description:"Path to a directory of PEM-encoded CA cert files to verify the vault server SSL cert."`
	ClientCert string `long:"client-cert"          description:"Path to the client certificate for Vault authorization."`
	ClientKey  string `long:"client-key"           description:"Path to the client private key for Vault authorization."`
	ServerName string `long:"server-name"          description:"If set, is used to set the SNI host when connecting via TLS."`
	Insecure   bool   `long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
}

type AuthConfig struct {
	ClientToken string `long:"client-token" description:"Client token for accessing secrets within the Vault server."`

	Backend       string        `long:"auth-backend"               description:"Auth backend to use for logging in to Vault."`
	BackendMaxTTL time.Duration `long:"auth-backend-max-ttl"       description:"Time after which to force a re-login. If not set, the token will just be continuously renewed."`
	RetryMax      time.Duration `long:"retry-max"     default:"5m" description:"The maximum time between retries when logging in or re-authing a secret."`
	RetryInitial  time.Duration `long:"retry-initial" default:"1s" description:"The initial time between retries when logging in or re-authing a secret."`

	Params map[string]string `long:"auth-param"  description:"Paramter to pass when logging in via the backend. Can be specified multiple times." value-name:"NAME:VALUE"`
}

func (manager *VaultManager) Init(log lager.Logger) error {
	var err error

	tlsConfig := &vaultapi.TLSConfig{
		CACert:        manager.TLS.CACert,
		CAPath:        manager.TLS.CAPath,
		TLSServerName: manager.TLS.ServerName,
		Insecure:      manager.TLS.Insecure,

		ClientCert: manager.TLS.ClientCert,
		ClientKey:  manager.TLS.ClientKey,
	}

	manager.Client, err = NewAPIClient(log, manager.URL, tlsConfig, manager.Auth)
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
		"ca_cert":            manager.TLS.CACert,
		"server_name":        manager.TLS.ServerName,
		"auth_backend":       manager.Auth.Backend,
		"auth_max_ttl":       manager.Auth.BackendMaxTTL,
		"auth_retry_max":     manager.Auth.RetryMax,
		"auth_retry_initial": manager.Auth.RetryInitial,
		"health":             health,
	})
}

func toString(s interface{}) string {
	if s == nil {
		return ""
	}
	return s.(string)
}

func toBool(s interface{}) bool {
	if s == nil {
		return false
	}
	return s.(bool)
}

func toDuration(s interface{}, defaultValue time.Duration) time.Duration {
	if s == nil {
		return defaultValue
	}
	return s.(time.Duration)
}

func (manager *VaultManager) Config(config map[string]interface{}) {
	manager.URL = toString(config["url"])
	manager.PathPrefix = toString(config["path_prefix"])
	manager.SharedPath = toString(config["shared_path"])

	manager.TLS.CACert = toString(config["ca_cert"])
	manager.TLS.CAPath = toString(config["ca_path"])
	manager.TLS.ClientCert = toString(config["client_cert"])
	manager.TLS.ClientKey = toString(config["client_key"])
	manager.TLS.ServerName = toString(config["server_name"])
	manager.TLS.Insecure = toBool(config["insecure_skip_verify"])

	manager.Auth.ClientToken = toString(config["client_token"])
	manager.Auth.Backend = toString(config["auth_backend"])
	manager.Auth.BackendMaxTTL = toDuration(config["auth_max_ttl"], 0)
	manager.Auth.RetryMax = toDuration(config["auth_retry_max"], 5*time.Minute)
	manager.Auth.RetryInitial = toDuration(config["auth_retry_initial"], 1*time.Second)
	if config["auth_param"] == nil {
		manager.Auth.Params = map[string]string{}
	} else {
		manager.Auth.Params = config["auth_param"].(map[string]string)
	}
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
		manager.ReAuther = NewReAuther(logger, manager.Client, manager.Auth.BackendMaxTTL, manager.Auth.RetryInitial, manager.Auth.RetryMax)
		manager.SecretFactory = NewVaultFactory(manager.Client, manager.ReAuther.LoggedIn(), manager.PathPrefix, manager.SharedPath)
	}
	return manager.SecretFactory, nil
}

func (manager VaultManager) Close(logger lager.Logger) {
	manager.ReAuther.Close()
}
