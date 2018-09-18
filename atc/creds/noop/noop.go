package noop

import "github.com/cloudfoundry/bosh-cli/director/template"

type Noop struct{}

func (n Noop) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	return nil, false, nil
}

func (n Noop) List() ([]template.VariableDefinition, error) {
	return []template.VariableDefinition{}, nil
}
