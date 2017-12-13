package ssm

import (
	"errors"
	"io/ioutil"
	"os"
	"text/template"
	"text/template/parse"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/concourse/atc/creds"
)

const DefaultPipeSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultTeamSecretTemplate = "/concourse/{{.Team}}/{{.Secret}}"

type SsmManager struct {
	AwsAccessKeyID     string `long:"aws-access-key" description:"AWS Access key ID"`
	AwsSecretAccessKey string `long:"aws-secret-key" description:"AWS Secret Access Key"`
	AwsSessionToken    string `long:"aws-session-token" description:"AWS Session Token"`
	AwsRegion          string `long:"aws-region" description:"AWS region to send requests to. Enviroment variable AWS_REGION is used if this flag is not provided."`
	PipeSecretTemplate string `long:"pipe-secret-template" description:"AWS SSM parameter name template used for pipeline specific paramter" default:"/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"`
	TeamSecretTemplate string `long:"team-secret-template" description:"AWS SSM parameter name template used for team specific paramter" default:"/concourse/{{.Team}}/{{.Secret}}"`
}

type SsmSecret struct {
	Team     string
	Pipeline string
	Secret   string
}

func buildSecretTemplate(name, tmpl string) (*template.Template, error) {
	t, err := template.
		New(name).
		Option("missingkey=error").
		Parse(tmpl)
	if err != nil {
		return nil, err
	}
	if parse.IsEmptyTree(t.Root) {
		return nil, errors.New("secret template should not be empty")
	}
	return t, nil
}

func (manager SsmManager) IsConfigured() bool {
	return manager.AwsRegion != "" || os.Getenv("AWS_REGION") != ""
}

func (manager SsmManager) Validate() error {
	// Make sure that the template is valid
	pipeSecretTemplate, err := buildSecretTemplate("pipe-secret-template", manager.PipeSecretTemplate)
	if err != nil {
		return err
	}
	teamSecretTemplate, err := buildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return err
	}
	// Execute the templates on dummy data to verify that it does not expect additional data
	dummy := SsmSecret{Team: "team", Pipeline: "pipeline", Secret: "secret"}
	if err = pipeSecretTemplate.Execute(ioutil.Discard, &dummy); err != nil {
		return err
	}
	if err = teamSecretTemplate.Execute(ioutil.Discard, &dummy); err != nil {
		return err
	}
	// All of the AWS credential variables may be empty since credentials may be obtained via environemnt variables
	// or other means. However, if one of them is provided, then all of them must be provided.
	if manager.AwsAccessKeyID == "" && manager.AwsSecretAccessKey == "" && manager.AwsSessionToken == "" {
		return nil
	}

	if manager.AwsAccessKeyID == "" {
		return errors.New("must provide aws access key id")
	}

	if manager.AwsSecretAccessKey == "" {
		return errors.New("must provide aws secret access key")
	}

	if manager.AwsSessionToken == "" {
		return errors.New("must provide aws session token")
	}

	return nil
}

func (manager SsmManager) NewVariablesFactory(log lager.Logger) (creds.VariablesFactory, error) {
	log.Info("Creating new SSM variables factory", lager.Data{
		"pipe-secret-template": manager.PipeSecretTemplate,
		"team-secret-template": manager.TeamSecretTemplate,
	})
	config := &aws.Config{}
	if manager.AwsRegion != "" {
		config.Region = &manager.AwsRegion
	}
	if manager.AwsAccessKeyID != "" {
		log.Info("Using AWS credentials provided by user", lager.Data{"aws-access-key": manager.AwsAccessKeyID})
		config.Credentials = credentials.NewStaticCredentials(manager.AwsAccessKeyID, manager.AwsSecretAccessKey, manager.AwsSessionToken)
	}

	session, err := session.NewSession(config)
	if err != nil {
		log.Error("Failed to establish AWS session", err)
		return nil, err
	}

	pipeSecretTemplate, err := buildSecretTemplate("pipe-secret-template", manager.PipeSecretTemplate)
	if err != nil {
		return nil, err
	}

	teamSecretTemplate, err := buildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return nil, err
	}

	return NewSsmFactory(log, session, []*template.Template{pipeSecretTemplate, teamSecretTemplate}), nil
}
