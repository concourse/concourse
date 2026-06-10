package secretsmanager

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/go-viper/mapstructure/v2"
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

func (factory *managerFactory) NewConfig() creds.ManagerConfig {
	return creds.ManagerConfig{
		Namespace:   "aws-secretsmanager",
		Description: "AWS SecretsManager Credential Management",
		Manager:     &Manager{},
	}
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

	return manager, nil
}
