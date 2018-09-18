package credhub

import (
	"github.com/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type credhubManagerFactory struct{}

func init() {
	creds.Register("credhub", NewCredHubManagerFactory())
}

func NewCredHubManagerFactory() creds.ManagerFactory {
	return &credhubManagerFactory{}
}

func (factory *credhubManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &CredHubManager{}

	subGroup, err := group.AddGroup("CredHub Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "credhub"

	return manager
}
