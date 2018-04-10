package secretsmanager

import (
	"errors"
	"io/ioutil"
	"text/template"
	"text/template/parse"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/concourse/atc/creds"
)

const DefaultPipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultTeamSecretTemplate = "/concourse/{{.Team}}/{{.Secret}}"

type Manager struct {
	AwsAccessKeyID         string `long:"access-key" description:"AWS Access key ID"`
	AwsSecretAccessKey     string `long:"secret-key" description:"AWS Secret Access Key"`
	AwsSessionToken        string `long:"session-token" description:"AWS Session Token"`
	AwsRegion              string `long:"region" description:"AWS region to send requests to" env:"AWS_REGION"`
	PipelineSecretTemplate string `long:"pipeline-secret-template" description:"AWS Manager secret identifier template used for pipeline specific parameter" default:"/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"`
	TeamSecretTemplate     string `long:"team-secret-template" description:"AWS SSM Manager secret identifier  template used for team specific parameter" default:"/concourse/{{.Team}}/{{.Secret}}"`
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

func (manager Manager) IsConfigured() bool {
	return manager.AwsRegion != ""
}

func (manager Manager) Validate() error {
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

	// All of the AWS credential variables may be empty since credentials may be obtained via environemnt variables
	// or other means. However, if one of them is provided, then all of them (except session token) must be provided.
	if manager.AwsAccessKeyID == "" && manager.AwsSecretAccessKey == "" && manager.AwsSessionToken == "" {
		return nil
	}

	if manager.AwsAccessKeyID == "" {
		return errors.New("must provide aws access key id")
	}

	if manager.AwsSecretAccessKey == "" {
		return errors.New("must provide aws secret access key")
	}

	return nil
}

func (manager Manager) NewVariablesFactory(log lager.Logger) (creds.VariablesFactory, error) {
	config := &aws.Config{Region: &manager.AwsRegion}
	if manager.AwsAccessKeyID != "" {
		config.Credentials = credentials.NewStaticCredentials(manager.AwsAccessKeyID, manager.AwsSecretAccessKey, manager.AwsSessionToken)
	}

	sess, err := session.NewSession(config)
	if err != nil {
		log.Error("create-aws-session", err)
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

	return NewSecretsManagerFactory(log, sess, []*template.Template{pipelineSecretTemplate, teamSecretTemplate}), nil
}
