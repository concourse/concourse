package vault

import (
	"fmt"

	"github.com/concourse/atc/creds"
	vaultapi "github.com/hashicorp/vault/api"
)

type vaultFactory struct {
	vaultClient vaultapi.Logical
}

func NewVaultFactory(v vaultapi.Logical) *vaultFactory {
	return &vaultFactory{
		vaultClient: v,
	}
}

func (v vaultFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return &Vault{
		PathPrefix:  fmt.Sprintf("%s/%s", teamName, pipelineName),
		VaultClient: &v.vaultClient,
	}
}
