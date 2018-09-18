package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc/creds"
	vaultapi "github.com/hashicorp/vault/api"
)

type VaultManager struct {
	URL string `long:"url" description:"Vault server address used to access secrets."`

	PathPrefix string `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`

	Cache    bool          `long:"cache" description:"Cache returned secrets for their lease duration in memory"`
	MaxLease time.Duration `long:"max-lease" description:"If the cache is enabled, and this is set, override secrets lease duration with a maximum value"`

	TLS    TLS
	Auth   AuthConfig
	Client *APIClient
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

	Params []template.VarKV `long:"auth-param"  description:"Paramter to pass when logging in via the backend. Can be specified multiple times." value-name:"NAME=VALUE"`
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
		"cache":              manager.Cache,
		"max_lease":          manager.MaxLease,
		"ca_cert":            manager.TLS.CACert,
		"server_name":        manager.TLS.ServerName,
		"auth_backend":       manager.Auth.Backend,
		"auth_max_ttl":       manager.Auth.BackendMaxTTL,
		"auth_retry_max":     manager.Auth.RetryMax,
		"auth_retry_initial": manager.Auth.RetryInitial,
		"health":             health,
	})
}

func (manager VaultManager) IsConfigured() bool {
	return manager.URL != ""
}

func (manager VaultManager) Validate() error {
	_, err := url.Parse(manager.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", err)
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

func (manager VaultManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	ra := NewReAuther(manager.Client, manager.Auth.BackendMaxTTL, manager.Auth.RetryInitial, manager.Auth.RetryMax)
	var sr SecretReader = manager.Client
	if manager.Cache {
		sr = NewCache(manager.Client, manager.MaxLease)
	}

	return NewVaultFactory(sr, ra.LoggedIn(), manager.PathPrefix), nil
}
