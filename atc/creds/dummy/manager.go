package dummy

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
)

const managerName = "dummy"

type Manager struct {
	Vars VarFlags `yaml:"var"`
}

func (manager *Manager) Name() string {
	return managerName
}

func (manager *Manager) Config() interface{} {
	return manager
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
