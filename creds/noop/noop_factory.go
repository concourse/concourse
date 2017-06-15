package noop

import "github.com/cloudfoundry/bosh-cli/director/template"

type noopFactory struct{}

func NewNoopFactory() *noopFactory {
	return &noopFactory{}
}

func (*noopFactory) NewVariables(string, string) template.Variables {
	return &Noop{}
}
