package credhub

import (
	"github.com/concourse/concourse/atc/creds"
)

type credhubManagerFactory struct{}

func NewCredHubManagerFactory() creds.ManagerFactory {
	return &credhubManagerFactory{}
}

func (factory *credhubManagerFactory) NewInstance(interface{}) (creds.Manager, error) {
	return &CredHubManager{}, nil
}
