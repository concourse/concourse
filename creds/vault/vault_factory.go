package vault

import (
	"path"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"

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

func (factory *vaultFactory) authLoop(logger lager.Logger, config AuthConfig) {
	for {
		currentToken := factory.currentToken()

		var token string
		var delay time.Duration
		if currentToken == "" {
			token, delay = factory.login(logger.Session("login"), config)
		} else {
			token, delay = factory.renew(logger.Session("renew"), currentToken)
		}

		if token != "" {
			factory.tokenL.Lock()
			factory.token = token
			if currentToken == "" {
				close(factory.loggedIn)
			}
			factory.tokenL.Unlock()
		}

		time.Sleep(delay)
	}
}

func (factory *vaultFactory) login(logger lager.Logger, config AuthConfig) (string, time.Duration) {
	if config.ClientToken != "" {
		return config.ClientToken, 0
	}

	backend := config.Backend

	params := map[string]interface{}{}
	for _, param := range config.Params {
		params[param.Name] = param.Value
	}

	secret, err := factory.vaultClient.Logical().Write(path.Join("auth", backend, "login"), params)
	if err != nil {
		logger.Error("failed", err)
		return "", time.Second
	}

	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	return secret.Auth.ClientToken, (time.Duration(secret.Auth.LeaseDuration) * time.Second) / 2
}

func (factory *vaultFactory) renew(logger lager.Logger, token string) (string, time.Duration) {
	secret, err := factory.clientWith(token).Auth().Token().RenewSelf(0)
	if err != nil {
		logger.Error("failed", err)
		return "", time.Second
	}

	logger.Info("succeeded", lager.Data{
		"token-accessor": secret.Auth.Accessor,
		"lease-duration": secret.Auth.LeaseDuration,
		"policies":       secret.Auth.Policies,
	})

	return secret.Auth.ClientToken, (time.Duration(secret.Auth.LeaseDuration) * time.Second) / 2
}

func (factory *vaultFactory) clientWith(token string) *vaultapi.Client {
	clientCopy := *factory.vaultClient
	vaultClient := &clientCopy
	vaultClient.SetToken(token)
	return vaultClient
}
