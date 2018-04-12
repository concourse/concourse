package vault

import (
	"path"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cenkalti/backoff"
	"github.com/concourse/atc/creds"
	vaultapi "github.com/hashicorp/vault/api"
)

type vaultFactory struct {
	vaultClient *vaultapi.Client

	prefix string

	token  string
	tokenL *sync.RWMutex

	loggedIn chan struct{}
}

func NewVaultFactory(logger lager.Logger, client *vaultapi.Client, auth AuthConfig, prefix string) *vaultFactory {
	factory := &vaultFactory{
		vaultClient: client,

		prefix: prefix,

		tokenL:   new(sync.RWMutex),
		loggedIn: make(chan struct{}),
	}

	go factory.authLoop(logger, auth)

	return factory
}

func (factory *vaultFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	<-factory.loggedIn

	return &Vault{
		VaultClient: factory.clientWith(factory.currentToken()).Logical(),

		PathPrefix:   factory.prefix,
		TeamName:     teamName,
		PipelineName: pipelineName,
	}
}

func (factory *vaultFactory) currentToken() string {
	factory.tokenL.RLock()
	token := factory.token
	factory.tokenL.RUnlock()
	return token
}

func (factory *vaultFactory) setToken(token string, lease time.Duration) {
	factory.tokenL.Lock()
	factory.token = token
	factory.tokenL.Unlock()
}

func (factory *vaultFactory) needsLogin(currentToken string, eol time.Time, lease time.Duration) bool {
	// never logged in
	if currentToken == "" {
		return true
	}

	// no EOL; no max TTL set
	if eol.IsZero() {
		return false
	}

	// we'll reach EOL before next renewal; force login
	if time.Now().Add(lease).After(eol) {
		return true
	}

	return false
}

func (factory *vaultFactory) authLoop(logger lager.Logger, config AuthConfig) {
	exp := backoff.NewExponentialBackOff()
	exp.MaxElapsedTime = 0

	var tokenEOL time.Time
	var lease time.Duration

	for {
		currentToken := factory.currentToken()

		logIn := factory.needsLogin(currentToken, tokenEOL, lease)

		var token string
		var authErr error
		if logIn {
			token, lease, authErr = factory.login(logger.Session("login"), config)
		} else {
			token, lease, authErr = factory.renew(logger.Session("renew"), currentToken)
		}

		if authErr != nil {
			time.Sleep(exp.NextBackOff())
			continue
		}

		if token != "" {
			if logIn && config.BackendMaxTTL > 0 {
				tokenEOL = time.Now().Add(config.BackendMaxTTL)
			}

			factory.setToken(token, lease)

			if currentToken == "" {
				close(factory.loggedIn)
			}
		}

		time.Sleep(lease / 2)
	}
}

func (factory *vaultFactory) login(logger lager.Logger, config AuthConfig) (string, time.Duration, error) {
	if config.ClientToken != "" {
		return config.ClientToken, time.Second, nil
	}

	backend := config.Backend

	params := map[string]interface{}{}
	for _, param := range config.Params {
		params[param.Name] = param.Value
	}

	secret, err := factory.vaultClient.Logical().Write(path.Join("auth", backend, "login"), params)
	if err != nil {
		logger.Error("failed", err)
		return "", 0, err
	}

	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	return secret.Auth.ClientToken, time.Duration(secret.Auth.LeaseDuration * int(time.Second)), nil
}

func (factory *vaultFactory) renew(logger lager.Logger, token string) (string, time.Duration, error) {
	secret, err := factory.clientWith(token).Auth().Token().RenewSelf(0)
	if err != nil {
		logger.Error("failed", err)
		return "", 0, err
	}

	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	return secret.Auth.ClientToken, time.Duration(secret.Auth.LeaseDuration * int(time.Second)), nil
}

func (factory *vaultFactory) clientWith(token string) *vaultapi.Client {
	clientCopy := *factory.vaultClient
	vaultClient := &clientCopy
	vaultClient.SetToken(token)
	return vaultClient
}
