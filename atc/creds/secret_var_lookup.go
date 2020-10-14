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

func (sl VariableLookupFromSecrets) Get(ref vars.Reference) (interface{}, bool, error) {
	val, found, err := sl.get(ref.Path)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	result, err := vars.Traverse(val, ref.Name, ref.Fields)
	if err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func (sl VariableLookupFromSecrets) get(path string) (interface{}, bool, error) {
	if len(sl.LookupPaths) == 0 {
		// if no paths are specified (i.e. for fake & noop secret managers), then try 1-to-1 var->secret mapping
		result, _, found, err := sl.Secrets.Get(path)
		return result, found, err
	}
	// try to find a secret according to our var->secret lookup paths
	for _, rule := range sl.LookupPaths {
		// prepends any additional prefix paths to front of the path
		secretPath, err := rule.VariableToSecretPath(path)
		if err != nil {
			return nil, false, err
		}
		result, _, found, err := sl.Secrets.Get(secretPath)
		if err != nil {
			return nil, false, err
		}
		if !found {
			continue
		}
		return result, true, nil
	}
	return nil, false, nil
}

func (sl VariableLookupFromSecrets) List() ([]vars.Reference, error) {
	return nil, nil
}
