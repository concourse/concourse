package vault

import (
	"errors"
	"fmt"
	"net/url"
	"path"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/creds"
	vaultapi "github.com/hashicorp/vault/api"
)

type VaultManager struct {
	URL string `long:"url" description:"Vault server address used to access secrets."`

	PathPrefix string `long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`

	TLS struct {
		CACert     string `long:"ca-cert"              description:"Path to a PEM-encoded CA cert file to use to verify the vault server SSL cert."`
		CAPath     string `long:"ca-path"              description:"Path to a directory of PEM-encoded CA cert files to verify the vault server SSL cert."`
		ServerName string `long:"server-name"          description:"If set, is used to set the SNI host when connecting via TLS."`
		Insecure   bool   `long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
	}

	Auth AuthConfig
}

type AuthConfig struct {
	Method string `long:"auth-method" description:"Auth method to use if no token is provided. Defaults to the backend for the auth specified, e.g. 'cert'."`

	ClientToken string `long:"client-token" description:"Vault client token for accessing secrets within the Vault server."`

	TLS struct {
		RoleName   string `long:"auth-tls-role-name"   description:"Role name to which to authenticate if a token is not specified."`
		ClientCert string `long:"auth-tls-client-cert" description:"Path to the certificate for Vault communication."`
		ClientKey  string `long:"auth-tls-client-key"  description:"Path to the private key for Vault communication."`
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

	if manager.Auth.ClientToken != "" {
		return nil
	}

	if manager.Auth.TLS.ClientCert != "" && manager.Auth.TLS.ClientKey != "" {
		return nil
	}

	if manager.Auth.TLS.ClientCert == "" && manager.Auth.TLS.ClientKey != "" {
		return errors.New("missing client cert")
	}

	if manager.Auth.TLS.ClientCert != "" && manager.Auth.TLS.ClientKey == "" {
		return errors.New("missing client key")
	}

	return errors.New("must configure client token or client cert/key")
}

func (manager VaultManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	config := vaultapi.DefaultConfig()

	err := config.ConfigureTLS(&vaultapi.TLSConfig{
		CACert:        manager.TLS.CACert,
		CAPath:        manager.TLS.CAPath,
		TLSServerName: manager.TLS.ServerName,
		Insecure:      manager.TLS.Insecure,

		ClientCert: manager.Auth.TLS.ClientCert,
		ClientKey:  manager.Auth.TLS.ClientKey,
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

	c := client.Logical()

	token := manager.Auth.ClientToken
	if token == "" {
		method := manager.Auth.Method
		params := map[string]interface{}{}

		if manager.Auth.TLS.ClientCert != "" && manager.Auth.TLS.ClientKey != "" {
			method = "cert"

			if manager.Auth.TLS.RoleName != "" {
				params["name"] = manager.Auth.TLS.RoleName
			}
		}

		secret, err := c.Write(path.Join("auth", method, "login"), params)
		if err != nil {
			return nil, fmt.Errorf("failed to log in to vault: %s", err)
		}

		logger.Info("logged-in-to-vault", lager.Data{
			"token-accessor": secret.Auth.Accessor,
			"lease-duration": secret.Auth.LeaseDuration,
			"policies":       secret.Auth.Policies,
		})

		token = secret.Auth.ClientToken
	}

	client.SetToken(token)

	return NewVaultFactory(*c, manager.PathPrefix), nil
}
