package creds

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type VarSources struct {
	varSources    atc.VarSourceConfigs
	varSourcePool VarSourcePool
	globalSecrets Secrets

	teamName     string
	pipelineName string

	logger lager.Logger
}

func NewVarSources(logger lager.Logger, varSources atc.VarSourceConfigs, varSourcePool VarSourcePool, globalSecrets Secrets, teamName, pipelineName string) *VarSources {
	return &VarSources{
		varSources:    varSources,
		varSourcePool: varSourcePool,
		globalSecrets: globalSecrets,
		teamName:      teamName,
		pipelineName:  pipelineName,
		logger:        logger,
	}
}

func (v *VarSources) Get(ref vars.Reference) (interface{}, bool, error) {
	// Var is evaluated by global credential manager
	if ref.Source == "" {
		globalVars := NewVariables(v.globalSecrets, v.teamName, v.pipelineName, false)
		return globalVars.Get(ref)
	}

	// Loop over each var source and try to match a var source to the source
	// provided in the var
	for _, varSource := range v.varSources {
		if ref.Source == varSource.Name {
			// Grab out the manager factory for that var source
			factory := ManagerFactories()[ref.Source]
			if factory == nil {
				return nil, false, fmt.Errorf("unknown credential manager type: %s", ref.Source)
			}

			config, ok := varSource.Config.(map[string]interface{})
			if !ok {
				return nil, false, fmt.Errorf("var_source '%s' invalid config", varSource.Name)
			}

			// Create a new var source fetcher that does not include the one we are
			// trying to evaluate. This will prevent the evaluation of the var source
			// configs to go in a loop.
			parentVarSources := NewVarSources(v.logger, v.varSources.Without(varSource.Name), v.varSourcePool, v.globalSecrets, v.teamName, v.pipelineName)

			// Evaluate the var source's config. If the config of the var source has
			// templated vars then it will end up recursing to evaluate the var
			// source config's vars until it is able to evaluate a source that does
			// not have any templated vars or is evaluated using the global
			// credential manager.
			evaluatedConfig, err := NewSource(parentVarSources, config).Evaluate()
			if err != nil {
				return nil, false, fmt.Errorf("evaluate: %w", err)
			}

			secrets, err := v.varSourcePool.FindOrCreate(v.logger, evaluatedConfig, factory)
			if err != nil {
				return nil, false, fmt.Errorf("find or create var source: %w", err)
			}

			lookupPaths := secrets.NewSecretLookupPaths(v.teamName, v.pipelineName, false)
			if len(lookupPaths) == 0 {
				// if no paths are specified (i.e. for fake & noop secret managers), then try 1-to-1 var->secret mapping
				result, _, found, err := secrets.Get(ref.Path)
				return result, found, err
			}
			// try to find a secret according to our var->secret lookup paths
			for _, rule := range lookupPaths {
				// prepends any additional prefix paths to front of the path
				secretPath, err := rule.VariableToSecretPath(ref.Path)
				if err != nil {
					return nil, false, err
				}

				result, _, found, err := secrets.Get(secretPath)
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
	}

	return nil, false, vars.MissingSourceError{Name: ref.String(), Source: ref.Source}
}
