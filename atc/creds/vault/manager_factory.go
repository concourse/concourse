package vault

import (
	"fmt"

	"github.com/concourse/concourse/atc/creds"
)

type vaultManagerFactory struct{}

func init() {
	creds.Register("vault", NewVaultManagerFactory())
}

func NewVaultManagerFactory() creds.ManagerFactory {
	return &vaultManagerFactory{}
}

func (factory *vaultManagerFactory) NewConfig() creds.ManagerConfig {
	return creds.ManagerConfig{
		Namespace:   "vault",
		Description: "Vault Credential Management",
		Manager:     &VaultManager{},
	}
}

func (factory *vaultManagerFactory) NewInstance(config any) (creds.Manager, error) {
	if c, ok := config.(map[string]any); !ok {
		return nil, fmt.Errorf("invalid vault config format")
	} else {
		manager := &VaultManager{}

		err := manager.Config(c)
		if err != nil {
			return nil, err
		}

		return manager, nil
	}
}
