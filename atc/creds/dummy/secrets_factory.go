package dummy

import (
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/vars"
)

type SecretsFactory struct {
	vars vars.StaticVariables
}

func NewSecretsFactory(flags []VarFlag) *SecretsFactory {
	vars := vars.StaticVariables{}
	for _, flag := range flags {
		vars[flag.Name] = flag.Value
	}

	return &SecretsFactory{
		vars: vars,
	}
}

func (factory *SecretsFactory) NewSecrets() creds.Secrets {
	return &Secrets{
		StaticVariables: factory.vars,
	}
}
