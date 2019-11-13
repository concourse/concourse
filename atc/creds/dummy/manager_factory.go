package dummy

import (
	"fmt"
	"github.com/concourse/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type managerFactory struct{}

func init() {
	creds.Register("dummy", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}

func (factory *managerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &Manager{}

	subGroup, err := group.AddGroup("Dummy Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "dummy-creds"

	return manager
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
