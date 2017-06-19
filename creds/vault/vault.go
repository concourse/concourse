package vault

import (
	"fmt"

	"github.com/cloudfoundry/bosh-cli/director/template"
	vaultapi "github.com/hashicorp/vault/api"
)

type Vault struct {
	PathPrefix  string
	VaultClient *vaultapi.Logical
}

func (v Vault) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	secret, err := v.VaultClient.Read(fmt.Sprintf("%s/%s", v.PathPrefix, varDef.Name))
	if err != nil || secret == nil {
		return nil, false, err
	}

	val, found := secret.Data["value"]
	if found {
		return val, true, nil
	}

	return secret.Data, true, nil
}

func (v Vault) List() ([]template.VariableDefinition, error) {
	// Don't think this works with vault.. if we need it to we'll figure it out
	// var defs []template.VariableDefinition

	// secret, err := v.vaultClient.List(v.PathPrefix)
	// if err != nil {
	// 	return defs, err
	// }

	// var def template.VariableDefinition
	// for name, _ := range secret.Data {
	// 	defs := append(defs, template.VariableDefinition{
	// 		Name: name,
	// 	})
	// }

	return []template.VariableDefinition{}, nil
}
