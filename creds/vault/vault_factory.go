package vault

import (
	"fmt"

	"github.com/cloudfoundry/bosh-cli/director/template"
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

func (v vaultFactory) NewVariables(teamName string, pipelineName string) template.Variables {
	return &Vault{
		PathPrefix:  fmt.Sprintf("%s/%s", teamName, pipelineName),
		VaultClient: &v.vaultClient,
	}
}
