package gcpsecretmanager

import (
	"errors"

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

	// Pipeline authors control var_source configs, so they must not be able
	// to choose a file on the web node for it to read.
	if manager.CredentialsFile != "" {
		return nil, errors.New("credentials_file is not supported in a var_source: use credentials_json or Application Default Credentials")
	}

	return manager, nil
}
