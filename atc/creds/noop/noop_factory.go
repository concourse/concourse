package noop

import "github.com/concourse/atc/creds"

type noopFactory struct{}

func NewNoopFactory() *noopFactory {
	return &noopFactory{}
}

func (*noopFactory) NewVariables(string, string) creds.Variables {
	return &Noop{}
}
