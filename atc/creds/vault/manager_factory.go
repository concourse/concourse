package vault

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/v8/atc/creds"
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

func (factory *vaultManagerFactory) NewInstance(config any) (creds.Manager, error) {
	c, ok := config.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid vault config format")
	}

	manager := &VaultManager{}
	err := manager.Config(c)
	if err != nil {
		return nil, err
	}

	if manager.URL == "" {
		return nil, errors.New("vault url must be specified")
	}

	return manager, nil
}
