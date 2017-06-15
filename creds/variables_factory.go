package creds

import "github.com/cloudfoundry/bosh-cli/director/template"

//go:generate counterfeiter . Variables

type Variables interface {
	Get(template.VariableDefinition) (interface{}, bool, error)
	List() ([]template.VariableDefinition, error)
}

//go:generate counterfeiter . VariablesFactory

type VariablesFactory interface {
	NewVariables(string, string) template.Variables
}
