package keyvault

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc/creds"
)

// keyVault is a Azure Key Vault implementation of the creds.Variable interface
type keyVault struct {
	log          lager.Logger
	PathPrefix   string
	TeamName     string
	PipelineName string

	reader SecretReader
}

// NewKeyVault returns a new KeyVault with the provided configuration
func NewKeyVault(log lager.Logger, reader SecretReader, prefix, teamName, pipelineName string) creds.Variables {
	return &keyVault{
		log:          log,
		PathPrefix:   prefix,
		TeamName:     teamName,
		PipelineName: pipelineName,
		reader:       reader,
	}
}

// Get implements the Variable interface
func (k *keyVault) Get(varDef varTemplate.VariableDefinition) (interface{}, bool, error) {
	value, found, err := k.reader.Get(k.path(k.PipelineName, varDef.Name))
	if err != nil {
		return nil, false, fmt.Errorf("error while trying to get secret %s: %s", varDef.Name, err)
	}

	if !found {
		value, found, err = k.reader.Get(k.path(varDef.Name))
		if err != nil {
			return nil, false, fmt.Errorf("error while trying to get secret %s: %s", varDef.Name, err)
		}

		// If still not found, return
		if !found {
			return nil, false, nil
		}
	}

	// If we got here, we are good to go, so return the value
	return value, true, nil
}

// List implements the Variable interface
func (k *keyVault) List() ([]varTemplate.VariableDefinition, error) {
	// not implemented, see vault implementation
	secrets, err := k.reader.List(k.PathPrefix)
	if err != nil {
		return nil, fmt.Errorf("unable to list secrets: %s", err)
	}
	variables := make([]varTemplate.VariableDefinition, len(secrets))
	for i := range secrets {
		variables[i] = varTemplate.VariableDefinition{
			Name: secrets[i],
		}
	}
	return variables, nil
}

func (k *keyVault) path(parts ...string) string {
	return fmt.Sprintf("%s-%s-%s", k.PathPrefix, k.TeamName, strings.Join(parts, "-"))
}
