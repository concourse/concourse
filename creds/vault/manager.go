package vault

import (
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

	Cache    bool          `bool:"cache" default:"false" description:"Cache returned secrets for their lease duration in memory"`
	MaxLease time.Duration `long:"max-lease" description:"If the cache is enabled, and this is set, override secrets lease duration with a maximum value"`

	TLS struct {
		CACert     string `long:"ca-cert"              description:"Path to a PEM-encoded CA cert file to use to verify the vault server SSL cert."`
		CAPath     string `long:"ca-path"              description:"Path to a directory of PEM-encoded CA cert files to verify the vault server SSL cert."`
		ClientCert string `long:"client-cert"          description:"Path to the client certificate for Vault authorization."`
		ClientKey  string `long:"client-key"           description:"Path to the client private key for Vault authorization."`
		ServerName string `long:"server-name"          description:"If set, is used to set the SNI host when connecting via TLS."`
		Insecure   bool   `long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
	}

	Auth AuthConfig
}

type AuthConfig struct {
	ClientToken string `long:"client-token" description:"Client token for accessing secrets within the Vault server."`

	Backend       string        `long:"auth-backend"               description:"Auth backend to use for logging in to Vault."`
	BackendMaxTTL time.Duration `long:"auth-backend-max-ttl"       description:"Time after which to force a re-login. If not set, the token will just be continuously renewed."`
	RetryMax      time.Duration `long:"retry-max"     default:"5m" description:"The maximum time between retries when logging in or re-authing a secret."`
	RetryInitial  time.Duration `long:"retry-initial" default:"1s" description:"The initial time between retries when logging in or re-authing a secret."`

	Params []template.VarKV `long:"auth-param"  description:"Paramter to pass when logging in via the backend. Can be specified multiple times." value-name:"NAME=VALUE"`
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

func (manager VaultManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	tlsConfig := &vaultapi.TLSConfig{
		CACert:        manager.TLS.CACert,
		CAPath:        manager.TLS.CAPath,
		TLSServerName: manager.TLS.ServerName,
		Insecure:      manager.TLS.Insecure,

		ClientCert: manager.TLS.ClientCert,
		ClientKey:  manager.TLS.ClientKey,
	}

	c, err := NewAPIClient(logger, manager.URL, tlsConfig, manager.Auth)
	if err != nil {
		return nil, err
	}

	ra := NewReAuther(c, manager.Auth.BackendMaxTTL, manager.Auth.RetryInitial, manager.Auth.RetryMax)
	var sr SecretReader = c
	if manager.Cache {
		sr = NewCache(c, manager.MaxLease)
	}

	return NewVaultFactory(sr, ra.LoggedIn(), manager.PathPrefix), nil
}
