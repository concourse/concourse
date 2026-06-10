package ssm

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/go-viper/mapstructure/v2"
)

type ssmManagerFactory struct{}

func init() {
	creds.Register("ssm", NewSsmManagerFactory())
}

func NewSsmManagerFactory() creds.ManagerFactory {
	return &ssmManagerFactory{}
}

func (factory *ssmManagerFactory) NewConfig() creds.ManagerConfig {
	return creds.ManagerConfig{
		Namespace:   "aws-ssm",
		Description: "AWS SSM Credential Management",
		Manager:     &SsmManager{},
	}
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

	return manager, nil
}
