package dummy

import (
	"fmt"

	"github.com/concourse/concourse/atc/creds"
)

type managerFactory struct{}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}

func (factory *managerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dummy credential manager config: %T", config)
	}

	vars, ok := configMap["vars"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid vars config: %T", configMap["vars"])
	}

	manager := &Manager{}

	for k, v := range vars {
		manager.Vars = append(manager.Vars, VarFlag{
			Name:  k,
			Value: v,
		})
	}

	return manager, nil
}
