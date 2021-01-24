package vault

import (
	"fmt"

	"github.com/concourse/concourse/atc/creds"
)

type vaultManagerFactory struct{}

func NewVaultManagerFactory() creds.ManagerFactory {
	return &vaultManagerFactory{}
}

func (factory *vaultManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	if c, ok := config.(map[string]interface{}); !ok {
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
