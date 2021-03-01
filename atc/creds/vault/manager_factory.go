package vault

import (
	"github.com/concourse/concourse/atc/creds"
)

func init() {
	creds.Register(managerName, NewVaultManagerFactory())
}

type vaultManagerFactory struct{}

func NewVaultManagerFactory() creds.ManagerFactory {
	return &vaultManagerFactory{}
}

func (factory *vaultManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	manager := &VaultManager{}

	err := manager.ApplyConfig(config)
	if err != nil {
		return nil, err
	}

	return manager, nil
}
