package secretsmanager

import (
	"github.com/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type managerFactory struct{}

func init() {
	creds.Register("secretsmanager", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}

func (factory *managerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &Manager{}
	subGroup, err := group.AddGroup("AWS SecretsManager Credential Management", "", manager)
	if err != nil {
		panic(err)
	}
	subGroup.Namespace = "aws-secretsmanager"
	return manager
}
