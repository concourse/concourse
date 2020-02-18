package dummy

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
)

type Manager struct {
	Vars []VarFlag `long:"var" description:"A YAML value to expose via credential management. Can be prefixed with a team and/or pipeline to limit scope." value-name:"[TEAM/[PIPELINE/]]VAR=VALUE"`
}

func (manager *Manager) Init(log lager.Logger) error {
	return nil
}

func (manager *Manager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]interface{}{
		"health": health,
	})
}

func (manager Manager) IsConfigured() bool {
	return len(manager.Vars) > 0
}

func (manager Manager) Validate() error {
	return nil
}

func (manager Manager) Health() (*creds.HealthResponse, error) {
	return &creds.HealthResponse{
		Method: "noop",
	}, nil
}

func (manager Manager) Close(logger lager.Logger) {

}

func (manager Manager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
	return NewSecretsFactory(manager.Vars), nil
}
