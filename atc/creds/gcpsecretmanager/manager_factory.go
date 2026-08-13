package gcpsecretmanager

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/go-viper/mapstructure/v2"
	flags "github.com/jessevdk/go-flags"
)

type managerFactory struct{}

func init() {
	creds.Register("gcpsecretmanager", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}

func (manager managerFactory) Health() (any, error) {
	return nil, nil
}

func (factory *managerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &Manager{}

	subGroup, err := group.AddGroup("GCP Secret Manager Credential Management", "", manager)
	if err != nil {
		panic(err)
	}
	subGroup.Namespace = "gcp-secretmanager"

	return manager
}

func (factory *managerFactory) NewInstance(config any) (creds.Manager, error) {
	manager := &Manager{
		SecretVersion:          DefaultSecretVersion,
		RequestTimeout:         DefaultRequestTimeout,
		PipelineSecretTemplate: DefaultPipelineSecretTemplate,
		TeamSecretTemplate:     DefaultTeamSecretTemplate,
		SharedSecretTemplate:   DefaultSharedSecretTemplate,
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      &manager,
		DecodeHook:  mapstructure.StringToTimeDurationHookFunc(), // decodes e.g. "10s"
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
