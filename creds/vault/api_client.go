package vault

import (
	"path"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager"
	vaultapi "github.com/hashicorp/vault/api"
)

// The APIClient is a SecretReader which maintains an authorized
// client using the Login and Renew functions.
type APIClient struct {
	clientValue *atomic.Value
	logger      lager.Logger
	config      AuthConfig
}

// NewAPIClient with the associated authorization config and underlying vault client.
func NewAPIClient(logger lager.Logger, client *vaultapi.Client, config AuthConfig) *APIClient {
	ac := &APIClient{
		clientValue: &atomic.Value{},
		logger:      logger,
		config:      config,
	}
	ac.setClient(client)
	return ac
}

// Read must be called after a successful login has occurred or an
// un-authorized client will be used.
func (ac *APIClient) Read(path string) (*vaultapi.Secret, error) {
	return ac.client().Logical().Read(path)
}

func (ac *APIClient) loginParams() map[string]interface{} {
	loginParams := make(map[string]interface{})
	for _, param := range ac.config.Params {
		loginParams[param.Name] = param.Value
	}

	return loginParams
}

// Login the APIClient using the credentials passed at
// construction. Returns a duration after which renew must be called.
func (ac *APIClient) Login() (time.Duration, error) {
	// If we are configured with a client token return right away
	// and trigger a renewal.
	if ac.config.ClientToken != "" {
		ac.setClient(clientWithToken(ac.client(), ac.config.ClientToken))
		return 0, nil
	}

	logger := ac.logger.Session("login")
	client := ac.client()
	secret, err := client.Logical().Write(path.Join("auth", ac.config.Backend, "login"), ac.loginParams())
	if err != nil {
		logger.Error("failed", err)
		return time.Second, err
	}
	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	ac.setClient(clientWithToken(client, secret.Auth.ClientToken))
	return time.Duration(secret.Auth.LeaseDuration) * time.Second, nil
}

// Renew the APIClient login using the credentials passed at
// construction. Must be called after a successful login. Returns a
// duration after which renew must be called again.
func (ac *APIClient) Renew() (time.Duration, error) {
	logger := ac.logger.Session("renew")
	client := ac.client()
	secret, err := client.Auth().Token().RenewSelf(0)
	if err != nil {
		logger.Error("failed", err)
		return time.Second, err
	}
	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	ac.setClient(clientWithToken(client, secret.Auth.ClientToken))
	return time.Duration(secret.Auth.LeaseDuration) * time.Second, nil
}

func (ac *APIClient) client() *vaultapi.Client {
	return ac.clientValue.Load().(*vaultapi.Client)
}

func (ac *APIClient) setClient(client *vaultapi.Client) {
	ac.clientValue.Store(client)
}

func clientWithToken(client *vaultapi.Client, token string) *vaultapi.Client {
	// TODO: This is dangerous, we are relying on the vault
	// client being ok with a shallow copy....
	// but...
	// the old code worked ;p
	clientCopy := *client
	newClient := &clientCopy
	newClient.SetToken(token)
	return newClient
}
