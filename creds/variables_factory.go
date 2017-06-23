package creds

import "github.com/cloudfoundry/bosh-cli/director/template"

//go:generate counterfeiter . VariablesFactory

type VariablesFactory interface {
	NewVariables(string, string) Variables
}

type Variables interface {
	Get(template.VariableDefinition) (interface{}, bool, error)
	List() ([]template.VariableDefinition, error)
}
