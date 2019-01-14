package keyvault

import (
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
}

// NewKeyVault returns a new KeyVault with the provided configuration
func NewKeyVault(log lager.Logger, prefix, teamName, pipelineName string) creds.Variables {
	return &keyVault{
		log:          log,
		PathPrefix:   prefix,
		TeamName:     teamName,
		PipelineName: pipelineName,
	}
}

// Get implements the Variable interface
func (s *keyVault) Get(varDef varTemplate.VariableDefinition) (interface{}, bool, error) {
	return nil, false, nil
}

// List implements the Variable interface
func (s *keyVault) List() ([]varTemplate.VariableDefinition, error) {
	// not implemented, see vault implementation
	return []varTemplate.VariableDefinition{}, nil
}
