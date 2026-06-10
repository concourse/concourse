package conjur

import (
	"github.com/concourse/concourse/atc/creds"
)

type managerFactory struct{}

func init() {
	creds.Register("conjur", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}
func (manager managerFactory) Health() (any, error) {
	return nil, nil
}

func (factory *managerFactory) NewConfig() creds.ManagerConfig {
	return creds.ManagerConfig{
		Namespace:   "conjur",
		Description: "Conjur Credential Management",
		Manager:     &Manager{},
	}
}

func (factory *managerFactory) NewInstance(any) (creds.Manager, error) {
	return &Manager{}, nil
}
