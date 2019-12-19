package conjur

import (
	"errors"
	"io/ioutil"
	"text/template"
	"text/template/parse"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-api-go/conjurapi/authn"
)

// Do not know if these constants are doing anything
// Defaults are grabbed from the 'default:' path in the Manager struct for PipelineSecretTemplate and TeamSecretTemplate

const DefaultPipelineSecretTemplate = "concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultTeamSecretTemplate = "concourse/{{.Team}}/{{.Secret}}"

type Manager struct {
	ConjurApplianceUrl     string `long:"appliance-url" description:"URL of the conjur instance"`
	ConjurAccount          string `long:"account" description:"Conjur Account"`
	ConjurCertFile         string `long:"cert-file" description:"Cert file used if conjur instance is using a self signed cert. E.g. /path/to/conjur.pem"`
	ConjurAuthnLogin       string `long:"authn-login" description:"Host username. E.g host/concourse"`
	ConjurAuthnApiKey      string `long:"authn-api-key" description:"Api key related to the host"`
	ConjurAuthnTokenFile   string `long:"authn-token-file" description:"Token file used if conjur instance is running in k8s or iam. E.g. /path/to/token_file"`
	PipelineSecretTemplate string `long:"pipeline-secret-template" description:"Conjur secret identifier template used for pipeline specific parameter" default:"concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"`
	TeamSecretTemplate     string `long:"team-secret-template" description:"Conjur secret identifier template used for team specific parameter" default:"concourse/{{.Team}}/{{.Secret}}"`
	SecretTemplate         string `long:"secret-template" description:"Conjur secret identifier template used for full path conjur secrets" default:"vaultName/{{.Secret}}"`
	Conjur                 *Conjur
}

type Secret struct {
	Team     string
	Pipeline string
	Secret   string
}

func buildSecretTemplate(name, tmpl string) (*template.Template, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	if parse.IsEmptyTree(t.Root) {
		return nil, errors.New("secret template should not be empty")
	}
	return t, nil
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
	// Make sure that the template is valid
	pipelineSecretTemplate, err := buildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate)
	if err != nil {
		return err
	}
	teamSecretTemplate, err := buildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return err
	}

	// Execute the templates on dummy data to verify that it does not expect additional data
	dummy := Secret{Team: "team", Pipeline: "pipeline", Secret: "secret"}
	if err = pipelineSecretTemplate.Execute(ioutil.Discard, &dummy); err != nil {
		return err
	}
	if err = teamSecretTemplate.Execute(ioutil.Discard, &dummy); err != nil {
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

	pipelineSecretTemplate, err := buildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate)
	if err != nil {
		return nil, err
	}

	teamSecretTemplate, err := buildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return nil, err
	}

	secretTemplate, err := buildSecretTemplate("secret-template", manager.SecretTemplate)
	if err != nil {
		return nil, err
	}

	return NewConjurFactory(log, client, []*template.Template{pipelineSecretTemplate, teamSecretTemplate, secretTemplate}), nil
}

func (manager Manager) Close(logger lager.Logger) {
	// TODO - to implement
}
