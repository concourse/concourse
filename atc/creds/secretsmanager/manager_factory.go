package secretsmanager

import (
	"github.com/concourse/concourse/atc/creds"
)

func init() {
	creds.Register(managerName, NewManagerFactory())
}

type managerFactory struct{}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}
func (manager managerFactory) Health() (interface{}, error) {
	return nil, nil
}

func (factory *managerFactory) NewInstance(interface{}) (creds.Manager, error) {
	return &Manager{}, nil
}
