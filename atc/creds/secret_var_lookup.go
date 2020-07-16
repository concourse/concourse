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
			// prepands any additionals prefix paths to front of the path
			secretRef, err := rule.VariableToSecretPath(varDef.Ref)
			if err != nil {
				return nil, false, err
			}
			result, _, found, err := sl.Secrets.Get(secretRef)
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
		result, _, found, err := sl.Secrets.Get(varDef.Ref)
		return result, found, err
	}
}

func (sl VariableLookupFromSecrets) List() ([]vars.VariableDefinition, error) {
	return nil, nil
}
