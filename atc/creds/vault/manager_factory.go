package vault

import (
	"fmt"
	"github.com/concourse/concourse/atc/creds"
	"github.com/jessevdk/go-flags"
)

type vaultManagerFactory struct{}

func init() {
	creds.Register("vault", NewVaultManagerFactory())
}

func NewVaultManagerFactory() creds.ManagerFactory {
	return &vaultManagerFactory{}
}

func (factory *vaultManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &VaultManager{}

	subGroup, err := group.AddGroup("Vault Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "vault"

	return manager
}

func (factory *vaultManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	if c, ok := config.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("invalid vault config format")
	} else {
		manager := &VaultManager{}
		manager.Config(c)
		return manager, nil
	}
}
