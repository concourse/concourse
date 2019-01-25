package dummy

import (
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
