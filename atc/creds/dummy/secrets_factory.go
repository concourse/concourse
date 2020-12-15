package dummy

import (
	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"
	"time"
)

type SecretsFactory struct {
	vars  vars.StaticVariables
	delay time.Duration
	clock clock.Clock
}

func NewSecretsFactory(flags []VarFlag, deploy time.Duration, clock clock.Clock) *SecretsFactory {
	vars := vars.StaticVariables{}
	for _, flag := range flags {
		vars[flag.Name] = flag.Value
	}

	return &SecretsFactory{
		vars:  vars,
		delay: deploy,
		clock: clock,
	}
}

func (factory *SecretsFactory) NewSecrets() creds.Secrets {
	if factory.delay > 0 && factory.clock != nil {
		timer := factory.clock.NewTimer(factory.delay)
		select {
		case <-timer.C():
		}
	}
	return &Secrets{
		StaticVariables: factory.vars,
	}
}
