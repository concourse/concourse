package ssm

import (
	"github.com/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type ssmManagerFactory struct{}

func init() {
	creds.Register("ssm", NewSsmManagerFactory())
}

func NewSsmManagerFactory() creds.ManagerFactory {
	return &ssmManagerFactory{}
}

func (factory *ssmManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &SsmManager{}
	subGroup, err := group.AddGroup("AWS SSM Credential Management", "", manager)
	if err != nil {
		panic(err)
	}
	subGroup.Namespace = "aws-ssm"
	return manager
}
