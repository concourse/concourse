package keyvault

import (
	"github.com/concourse/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type keyVaultManagerFactory struct{}

func init() {
	creds.Register("keyvault", NewKeyVaultManagerFactory())
}

// NewKeyVaultManagerFactory returns the Azure Key Vault implementation of the
// ManagerFactory interface
func NewKeyVaultManagerFactory() creds.ManagerFactory {
	return &keyVaultManagerFactory{}
}

// AddConfig implements the ManagerFactory interface and returns an Azure Key
// Vault manager implementation
func (factory *keyVaultManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &KeyVaultManager{}
	subGroup, err := group.AddGroup("Azure Key Vault Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "keyvault"
	return manager
}
