package vault

import (
	"github.com/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
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
