package secretsmanager

import (
	"errors"

	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/go-viper/mapstructure/v2"
	flags "github.com/jessevdk/go-flags"
)

type managerFactory struct{}

func init() {
	creds.Register("secretsmanager", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}
func (manager managerFactory) Health() (any, error) {
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

func (factory *managerFactory) NewInstance(config any) (creds.Manager, error) {
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

	if manager.AwsRegion == "" {
		return nil, errors.New("region must be specified")
	}

	if manager.AwsAccessKeyID == "" || manager.AwsSecretAccessKey == "" {
		return nil, errors.New("access_key and secret_key must be specified")
	}

	return manager, nil
}
