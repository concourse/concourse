package conjur

import (
	"github.com/concourse/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type managerFactory struct{}

func init() {
	creds.Register("conjur", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}
func (manager managerFactory) Health() (interface{}, error) {
	return nil, nil
}

func (factory *managerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &Manager{}
	subGroup, err := group.AddGroup("Conjur Credential Management", "", manager)
	if err != nil {
		panic(err)
	}
	subGroup.Namespace = "conjur"
	return manager
}

func (factory *managerFactory) NewInstance(interface{}) (creds.Manager, error) {
	return &Manager{}, nil
}
