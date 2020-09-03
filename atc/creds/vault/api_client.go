package vault

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/go-rootcerts"
	vaultapi "github.com/hashicorp/vault/api"
)

// The APIClient is a SecretReader which maintains an authorized
// client using the Login and Renew functions.
type APIClient struct {
	logger lager.Logger

	apiURL     string
	namespace  string
	tlsConfig  TLSConfig
	authConfig AuthConfig

	clientValue *atomic.Value

	renewable bool
}

// NewAPIClient with the associated authorization config and underlying vault client.
func NewAPIClient(logger lager.Logger, apiURL string, tlsConfig TLSConfig, authConfig AuthConfig, namespace string) (*APIClient, error) {
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
	// Check if path is kv1 or kv2
	path = sanitizePath(path)
	mountPath, kv2, err := isKVv2(path, ac.client())
	if err != nil {
		return nil, err
	}

	// If the path is under a kv2 mount, add the /data/ path to the prefix
	if kv2 {
		path = addPrefixToVKVPath(path, mountPath, "data")
	}

	secret, err := ac.client().Logical().Read(path)
	if err != nil || secret == nil {
		return secret, err
	}

	// Need to discard the metadata object and pull the v2 data field up to match kv1
	if kv2 {
		if data, ok := secret.Data["data"]; ok && data != nil {
			secret.Data = data.(map[string]interface{})
		} else {
			// Return a nil secret object if the secret was deleted, but not destroyed
			return nil, nil
		}
	}

	return secret, err
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

	if ac.authConfig.Backend == "ldap" || ac.authConfig.Backend == "okta" {
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

	err := ac.configureTLS(config.HttpClient.Transport.(*http.Transport).TLSClientConfig)
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

func (ac *APIClient) configureTLS(config *tls.Config) error {
	if ac.tlsConfig.CACert != "" || ac.tlsConfig.CACertFile != "" || ac.tlsConfig.CAPath != "" {
		rootConfig := &rootcerts.Config{
			CAFile:        ac.tlsConfig.CACertFile,
			CAPath:        ac.tlsConfig.CAPath,
			CACertificate: []byte(ac.tlsConfig.CACert),
		}

		if err := rootcerts.ConfigureTLS(config, rootConfig); err != nil {
			return err
		}
	}

	if ac.tlsConfig.ClientCertFile != "" {
		content, err := ioutil.ReadFile(ac.tlsConfig.ClientCertFile)
		if err != nil {
			return err
		}

		ac.tlsConfig.ClientCert = string(content)
	}

	if ac.tlsConfig.ClientKeyFile != "" {
		content, err := ioutil.ReadFile(ac.tlsConfig.ClientKeyFile)
		if err != nil {
			return err
		}

		ac.tlsConfig.ClientKey = string(content)
	}

	if ac.tlsConfig.Insecure {
		config.InsecureSkipVerify = true
	}

	var clientCert tls.Certificate
	foundClientCert := false

	switch {
	case ac.tlsConfig.ClientCert != "" && ac.tlsConfig.ClientKey != "":
		var err error
		clientCert, err = tls.X509KeyPair([]byte(ac.tlsConfig.ClientCert), []byte(ac.tlsConfig.ClientKey))
		if err != nil {
			return err
		}

		foundClientCert = true
	case ac.tlsConfig.ClientCert != "" || ac.tlsConfig.ClientKey != "":
		return fmt.Errorf("both client cert and client key must be provided")
	}

	if foundClientCert {
		// We use this function to ignore the server's preferential list of
		// CAs, otherwise any CA used for the cert auth backend must be in the
		// server's CA pool
		config.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &clientCert, nil
		}
	}

	if ac.tlsConfig.ServerName != "" {
		config.ServerName = ac.tlsConfig.ServerName
	}

	return nil
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
