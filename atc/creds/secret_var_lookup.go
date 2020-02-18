package creds

import (
	"github.com/concourse/concourse/vars"
)

type VariableLookupFromSecrets struct {
	Secrets     Secrets
	LookupPaths []SecretLookupPath
}

func NewVariables(secrets Secrets, teamName string, pipelineName string, allowRootPath bool) vars.Variables {
	return VariableLookupFromSecrets{
		Secrets:     secrets,
		LookupPaths: secrets.NewSecretLookupPaths(teamName, pipelineName, allowRootPath),
	}
}

func (sl VariableLookupFromSecrets) Get(varDef vars.VariableDefinition) (interface{}, bool, error) {
	// try to find a secret according to our var->secret lookup paths
	if len(sl.LookupPaths) > 0 {
		for _, rule := range sl.LookupPaths {
			secretId, err := rule.VariableToSecretPath(varDef.Name)
			if err != nil {
				return nil, false, err
			}
			result, _, found, err := sl.Secrets.Get(secretId)
			if err != nil {
				return nil, false, err
			}
			if !found {
				continue
			}
			return result, true, nil
		}
		return nil, false, nil
	} else {
		// if no paths are specified (i.e. for fake & noop secret managers), then try 1-to-1 var->secret mapping
		result, _, found, err := sl.Secrets.Get(varDef.Name)
		return result, found, err
	}
}

func (sl VariableLookupFromSecrets) List() ([]vars.VariableDefinition, error) {
	return nil, nil
}
