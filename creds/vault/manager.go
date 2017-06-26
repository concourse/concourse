package vault

import (
	"errors"
	"fmt"
	"net/url"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc/creds"
	vaultapi "github.com/hashicorp/vault/api"
)

type VaultManager struct {
	URL string `long:"url" description:"Vault server address used to access secrets."`

	PathPrefix string `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`

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

	Backend string           `long:"auth-backend" description:"Auth backend to use for logging in to Vault."`
	Params  []template.VarKV `long:"auth-param"  description:"Paramter to pass when logging in via the backend. Can be specified multiple times." value-name:"NAME=VALUE"`
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
	config := vaultapi.DefaultConfig()

	err := config.ConfigureTLS(&vaultapi.TLSConfig{
		CACert:        manager.TLS.CACert,
		CAPath:        manager.TLS.CAPath,
		TLSServerName: manager.TLS.ServerName,
		Insecure:      manager.TLS.Insecure,

		ClientCert: manager.TLS.ClientCert,
		ClientKey:  manager.TLS.ClientKey,
	})
	if err != nil {
		return nil, err
	}

	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, err
	}

	err = client.SetAddress(manager.URL)
	if err != nil {
		return nil, err
	}

	return NewVaultFactory(logger, client, manager.Auth, manager.PathPrefix), nil
}
