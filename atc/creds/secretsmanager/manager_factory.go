package secretsmanager

import (
	"github.com/concourse/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
	"github.com/mitchellh/mapstructure"
)

type managerFactory struct{}

func init() {
	creds.Register("secretsmanager", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}
func (manager managerFactory) Health() (interface{}, error) {
	return nil, nil
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

func (factory *managerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	manager := &Manager{
		TeamSecretTemplate:     DefaultTeamSecretTemplate,
		SharedSecretTemplate:   DefaultSharedSecretTemplate,
		PipelineSecretTemplate: DefaultPipelineSecretTemplate,
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      &manager,
	})
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return manager, nil
}
