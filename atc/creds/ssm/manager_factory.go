package ssm

import (
	"errors"

	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/go-viper/mapstructure/v2"
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

func (factory *ssmManagerFactory) NewInstance(config any) (creds.Manager, error) {
	manager := &SsmManager{
		TeamSecretTemplate:     DefaultTeamSecretTemplate,
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
