package vault

import (
	"github.com/concourse/atc/creds"
	vaultapi "github.com/hashicorp/vault/api"
)

type vaultFactory struct {
	vaultClient vaultapi.Logical
	prefix      string
}

func NewVaultFactory(v vaultapi.Logical, prefix string) *vaultFactory {
	return &vaultFactory{
		vaultClient: v,
		prefix:      prefix,
	}
}

func (v vaultFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return &Vault{
		VaultClient: &v.vaultClient,

		PathPrefix:   v.prefix,
		TeamName:     teamName,
		PipelineName: pipelineName,
	}
}
