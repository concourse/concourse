package creds

import (
	"code.cloudfoundry.org/lager/v3"
)

type Manager interface {
	IsConfigured() bool
	Validate() error
	Health() (*HealthResponse, error)
	Init(lager.Logger) error
	Close(logger lager.Logger)

	NewSecretsFactory(lager.Logger) (SecretsFactory, error)
}

type ManagerFactory interface {
	NewInstance(any) (Manager, error)

	// NewConfig returns a fresh flag-bearing manager for command-line
	// configuration, along with the flag namespace its fields live under.
	// An empty namespace means the manager exposes no flags.
	NewConfig() ManagerConfig
}

type ManagerConfig struct {
	// Namespace prefixes every flag of the manager, e.g. "vault" for
	// --vault-url and CONCOURSE_VAULT_URL. Empty means no flags.
	Namespace string

	// Description is the heading the manager's flags appear under in
	// --help output, e.g. "Vault Credential Management".
	Description string

	Manager Manager
}

type Managers map[string]Manager

type CredentialManagementConfig struct {
	RetryConfig SecretRetryConfig
	CacheConfig SecretCacheConfig
}

// NewSecrets creates a Secrets object from secretsFactory based on configs.
func (c CredentialManagementConfig) NewSecrets(secretsFactory SecretsFactory) Secrets {
	result := secretsFactory.NewSecrets()
	result = NewRetryableSecrets(result, c.RetryConfig)
	if c.CacheConfig.Enabled {
		result = NewCachedSecrets(result, c.CacheConfig)
	}
	return result
}

type HealthResponse struct {
	Response any    `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
	Method   string `json:"method,omitempty"`
}

var managerFactories = map[string]ManagerFactory{}

func Register(name string, managerFactory ManagerFactory) {
	managerFactories[name] = managerFactory
}

func ManagerFactories() map[string]ManagerFactory {
	return managerFactories
}
