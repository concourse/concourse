package credhub

import (
	"github.com/concourse/concourse/atc/creds"
)

type credhubManagerFactory struct{}

func init() {
	creds.Register("credhub", NewCredHubManagerFactory())
}

func NewCredHubManagerFactory() creds.ManagerFactory {
	return &credhubManagerFactory{}
}

func (factory *credhubManagerFactory) NewConfig() creds.ManagerConfig {
	return creds.ManagerConfig{
		Namespace:   "credhub",
		Description: "CredHub Credential Management",
		Manager:     &CredHubManager{},
	}
}

func (factory *credhubManagerFactory) NewInstance(any) (creds.Manager, error) {
	return &CredHubManager{}, nil
}
