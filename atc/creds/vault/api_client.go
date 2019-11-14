package vault

import (
	"fmt"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager"
	vaultapi "github.com/hashicorp/vault/api"
)

// The APIClient is a SecretReader which maintains an authorized
// client using the Login and Renew functions.
type APIClient struct {
	logger lager.Logger

	apiURL     string
	namespace  string
	tlsConfig  *vaultapi.TLSConfig
	authConfig AuthConfig

	clientValue *atomic.Value

	renewable bool
}

// NewAPIClient with the associated authorization config and underlying vault client.
func NewAPIClient(logger lager.Logger, apiURL string, tlsConfig *vaultapi.TLSConfig, authConfig AuthConfig, namespace string) (*APIClient, error) {
	ac := &APIClient{
		logger: logger,

		apiURL:     apiURL,
		namespace:  namespace,
		tlsConfig:  tlsConfig,
		authConfig: authConfig,

		clientValue: &atomic.Value{},

		renewable: true,
	}

	client, err := ac.baseClient()
	if err != nil {
		return nil, err
	}

	ac.setClient(client)

	return ac, nil
}

// Read must be called after a successful login has occurred or an
// un-authorized client will be used.
func (ac *APIClient) Read(path string) (*vaultapi.Secret, error) {
	return ac.client().Logical().Read(path)
}

func (ac *APIClient) loginParams() map[string]interface{} {
	loginParams := make(map[string]interface{})
	for k, v := range ac.authConfig.Params {
		loginParams[k] = v
	}

	return loginParams
}

// Login the APIClient using the credentials passed at
// construction. Returns a duration after which renew must be called.
func (ac *APIClient) Login() (time.Duration, error) {
	logger := ac.logger.Session("login")

	// If we are configured with a client token return right away
	// and trigger a renewal.
	if ac.authConfig.ClientToken != "" {
		newClient, err := ac.clientWithToken(ac.authConfig.ClientToken)
		if err != nil {
			logger.Error("failed-to-create-client", err)
			return time.Second, err
		}

		ac.setClient(newClient)

		logger.Info("token-set")

		return time.Second, nil
	}

	client := ac.client()
	loginPath := path.Join("auth", ac.authConfig.Backend, "login")

	if ac.authConfig.Backend == "ldap" {
		username, ok := ac.loginParams()["username"].(string)
		if !ok {
			err := fmt.Errorf("failed to assert username as string")
			logger.Error("failed", err)
			return time.Second, err
		}
		loginPath = path.Join("auth", ac.authConfig.Backend, "login", username)
	}

	secret, err := client.Logical().Write(loginPath, ac.loginParams())
	if err != nil {
		logger.Error("failed", err)
		return time.Second, err
	}

	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	newClient, err := ac.clientWithToken(secret.Auth.ClientToken)
	if err != nil {
		logger.Error("failed-to-create-client", err)
		return time.Second, err
	}

	ac.setClient(newClient)

	return time.Duration(secret.Auth.LeaseDuration) * time.Second, nil
}

// Renew the APIClient login using the credentials passed at
// construction. Must be called after a successful login. Returns a
// duration after which renew must be called again.
func (ac *APIClient) Renew() (time.Duration, error) {
	if !ac.renewable {
		return time.Second, nil
	}

	logger := ac.logger.Session("renew")

	client := ac.client()

	secret, err := client.Auth().Token().RenewSelf(0)
	if err != nil {
		// When tests with a Vault dev server, renew is not allowed.
		if strings.Index(err.Error(), "lease is not renewable") > 0 {
			ac.renewable = false
			return time.Second, nil
		}
		logger.Error("failed", err)
		return time.Second, err
	}

	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	newClient, err := ac.clientWithToken(secret.Auth.ClientToken)
	if err != nil {
		logger.Error("failed-to-create-client", err)
		return time.Second, err
	}

	ac.setClient(newClient)

	return time.Duration(secret.Auth.LeaseDuration) * time.Second, nil
}

func (ac *APIClient) client() *vaultapi.Client {
	return ac.clientValue.Load().(*vaultapi.Client)
}

func (ac *APIClient) setClient(client *vaultapi.Client) {
	ac.clientValue.Store(client)
}

func (ac *APIClient) baseClient() (*vaultapi.Client, error) {
	config := vaultapi.DefaultConfig()

	err := config.ConfigureTLS(ac.tlsConfig)
	if err != nil {
		return nil, err
	}

	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, err
	}

	err = client.SetAddress(ac.apiURL)
	if err != nil {
		return nil, err
	}

	if ac.namespace != "" {
		client.SetNamespace(ac.namespace)
	}

	return client, nil
}

func (ac *APIClient) clientWithToken(token string) (*vaultapi.Client, error) {
	client, err := ac.baseClient()
	if err != nil {
		return nil, err
	}

	client.SetToken(token)

	return client, nil
}

func (ac *APIClient) health() (*vaultapi.HealthResponse, error) {
	client, err := ac.baseClient()
	if err != nil {
		return nil, err
	}

	healthResponse, err := client.Sys().Health()
	return healthResponse, err
}
