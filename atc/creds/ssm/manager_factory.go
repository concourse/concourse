package ssm

import (
	"fmt"
	"github.com/concourse/concourse/atc/creds"
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

func (factory *ssmManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	if c, ok := config.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("invalid aws ssm config format")
	} else {
		manager := &SsmManager{}

		err := manager.Config(c)
		if err != nil {
			return nil, err
		}

		return manager, nil
	}
}
