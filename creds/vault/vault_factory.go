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

	token        string
	tokenEndLife time.Time
	lease        time.Duration
	tokenL       *sync.RWMutex

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

func (factory *vaultFactory) isTokenRenewable(config AuthConfig) bool {
	if factory.currentToken() != "" && (factory.tokenEndLife.Sub(time.Now()).Seconds()/factory.lease.Seconds()) > 1 {
		return true
	}
	return false
}

func (factory *vaultFactory) registerToken(currentToken string, token string, lease time.Duration, config AuthConfig, updateEOL bool) {
	if token == "" {
		return
	}
	factory.tokenL.Lock()
	factory.token = token
	if updateEOL {
		factory.tokenEndLife = time.Now().Add(config.BackendMaxTTL)
	}
	factory.lease = lease
	if currentToken == "" {
		close(factory.loggedIn)
	}
	factory.tokenL.Unlock()
}

func (factory *vaultFactory) authLoop(logger lager.Logger, config AuthConfig) {
	for {
		currentToken := factory.currentToken()

		var token string
		var lease time.Duration
		if factory.isTokenRenewable(config) {
			token, lease = factory.renew(logger.Session("renew"), currentToken)
			factory.registerToken(currentToken, token, lease, config, false)
		} else {
			token, lease = factory.login(logger.Session("login"), config)
			factory.registerToken(currentToken, token, lease, config, true)
		}

		time.Sleep(lease / 2)
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

	return secret.Auth.ClientToken, time.Duration(secret.Auth.LeaseDuration * int(time.Second))
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

	return secret.Auth.ClientToken, time.Duration(secret.Auth.LeaseDuration * int(time.Second))
}

func (factory *vaultFactory) clientWith(token string) *vaultapi.Client {
	clientCopy := *factory.vaultClient
	vaultClient := &clientCopy
	vaultClient.SetToken(token)
	return vaultClient
}
