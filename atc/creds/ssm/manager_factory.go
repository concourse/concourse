package ssm

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/mitchellh/mapstructure"
)

func init() {
	creds.Register(managerName, NewSsmManagerFactory())
}

type ssmManagerFactory struct{}

func NewSsmManagerFactory() creds.ManagerFactory {
	return &ssmManagerFactory{}
}

func (factory *ssmManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
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
