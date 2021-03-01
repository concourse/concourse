package conjur

import (
	"errors"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-api-go/conjurapi/authn"
)

// Do not know if these constants are doing anything
// Defaults are grabbed from the 'default:' path in the Manager struct for PipelineSecretTemplate and TeamSecretTemplate

const managerName = "conjur"

const DefaultPipelineSecretTemplate = "concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultTeamSecretTemplate = "concourse/{{.Team}}/{{.Secret}}"

type Manager struct {
	ConjurApplianceUrl     string `yaml:"appliance_url,omitempty"`
	ConjurAccount          string `yaml:"account,omitempty"`
	ConjurCertFile         string `yaml:"cert_file,omitempty" validate:"file"`
	ConjurAuthnLogin       string `yaml:"authn_login,omitempty"`
	ConjurAuthnApiKey      string `yaml:"authn_api_key,omitempty"`
	ConjurAuthnTokenFile   string `yaml:"authn_token_file,omitempty" validate:"file"`
	PipelineSecretTemplate string `yaml:"pipeline_secret_template,omitempty"`
	TeamSecretTemplate     string `yaml:"team_secret_template,omitempty"`
	SecretTemplate         string `yaml:"secret_template,omitempty"`
	Conjur                 *Conjur
}

func newConjurClient(manager *Manager) (*conjurapi.Client, error) {
	config := conjurapi.Config{
		Account:      manager.ConjurAccount,
		ApplianceURL: manager.ConjurApplianceUrl,
		SSLCertPath:  manager.ConjurCertFile,
	}

	if manager.ConjurAuthnTokenFile != "" {
		return conjurapi.NewClientFromTokenFile(
			config,
			manager.ConjurAuthnTokenFile,
		)
	}

	return conjurapi.NewClientFromKey(config,
		authn.LoginPair{
			Login:  manager.ConjurAuthnLogin,
			APIKey: manager.ConjurAuthnApiKey,
		},
	)
}

func (manager *Manager) Name() string {
	return managerName
}

func (manager *Manager) Config() interface{} {
	return manager
}

func (manager *Manager) Init(log lager.Logger) error {
	conjur, err := newConjurClient(manager)
	if err != nil {
		log.Error("create-conjur-api-instance", err)
		return err
	}

	manager.Conjur = &Conjur{
		log:    log,
		client: conjur,
	}

	return nil
}

func (manager *Manager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "GetSecretValue",
	}

	health.Response = map[string]string{
		"status": "UP",
	}

	return health, nil
}

func (manager *Manager) IsConfigured() bool {
	return manager.ConjurApplianceUrl != ""
}

func (manager *Manager) Validate() error {
	if _, err := creds.BuildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate); err != nil {
		return err
	}

	if _, err := creds.BuildSecretTemplate("team-secret-template", manager.TeamSecretTemplate); err != nil {
		return err
	}

	if _, err := creds.BuildSecretTemplate("secret-template", manager.SecretTemplate); err != nil {
		return err
	}

	if manager.ConjurApplianceUrl == "" {
		return errors.New("must provide conjur appliance url")
	}

	if manager.ConjurAccount == "" {
		return errors.New("must provide conjur account")
	}

	if manager.ConjurAuthnLogin == "" {
		return errors.New("must provide conjur login")
	}

	if manager.ConjurAuthnApiKey == "" && manager.ConjurAuthnTokenFile == "" {
		return errors.New("must provide conjur authn key or conjur authn token file")
	}

	if manager.ConjurAuthnApiKey != "" && manager.ConjurAuthnTokenFile != "" {
		return errors.New("must provide conjur authn key or conjur authn token file")
	}

	return nil
}

func (manager *Manager) NewSecretsFactory(log lager.Logger) (creds.SecretsFactory, error) {
	client, err := newConjurClient(manager)

	if err != nil {
		log.Error("create-conjur-api-instance", err)
		return nil, err
	}

	pipelineSecretTemplate, err := creds.BuildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate)
	if err != nil {
		return nil, err
	}

	teamSecretTemplate, err := creds.BuildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return nil, err
	}

	secretTemplate, err := creds.BuildSecretTemplate("secret-template", manager.SecretTemplate)
	if err != nil {
		return nil, err
	}

	return NewConjurFactory(log, client, []*creds.SecretTemplate{pipelineSecretTemplate, teamSecretTemplate, secretTemplate}), nil
}

func (manager Manager) Close(logger lager.Logger) {
	// TODO - to implement
}
