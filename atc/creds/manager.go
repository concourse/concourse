package creds

import (
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Manager

type Manager interface {
	Name() string
	Config() interface{}

	IsConfigured() bool
	Validate() error
	Health() (*HealthResponse, error)
	Init(lager.Logger) error
	Close(logger lager.Logger)

	NewSecretsFactory(lager.Logger) (SecretsFactory, error)
}

type ManagerFactory interface {
	NewInstance(interface{}) (Manager, error)
}

type Managers map[string]Manager

type CredentialManagementConfig struct {
	RetryConfig SecretRetryConfig `yaml:"secret_retry"`
	CacheConfig SecretCacheConfig `yaml:"secret_cache"`
}

type HealthResponse struct {
	Response interface{} `json:"response,omitempty"`
	Error    string      `json:"error,omitempty"`
	Method   string      `json:"method,omitempty"`
}

var managerFactories = map[string]ManagerFactory{}

func Register(name string, managerFactory ManagerFactory) {
	managerFactories[name] = managerFactory
}

func ManagerFactories() map[string]ManagerFactory {
	return managerFactories
}
