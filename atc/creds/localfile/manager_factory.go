package localfile

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/jessevdk/go-flags"
)

type ManagerFactory struct{}

func init() {
	creds.Register("localfile", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &ManagerFactory{}
}

func (factory *ManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &Manager{}

	subGroup, err := group.AddGroup("Local File Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "localfile"

	return manager
}

func (factory *ManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	return &Manager{}, nil
}
