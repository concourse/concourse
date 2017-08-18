package credhub

import (
	"github.com/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type credhubManagerFactory struct{}

func init() {
	creds.Register("credhub", NewCredhubManagerFactory())
}

func NewCredhubManagerFactory() creds.ManagerFactory {
	return &credhubManagerFactory{}
}

func (factory *credhubManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &CredhubManager{}

	subGroup, err := group.AddGroup("Credhub Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "credhub"

	return manager
}
