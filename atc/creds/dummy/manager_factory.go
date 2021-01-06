package dummy

import (
	"code.cloudfoundry.org/clock"
	"fmt"
	flags "github.com/jessevdk/go-flags"
	"time"

	"github.com/concourse/concourse/atc/creds"
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

	// deploy is a feature for simulating a credential manager's login duration.
	if delay, ok := configMap["delay"]; ok {
		manager.delay = delay.(time.Duration)
		manager.clock = configMap["clock"].(clock.Clock)
	}

	return manager, nil
}
