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

const DefaultSecretTemplate = "concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultFallbackTemplate = "concourse/{{.Team}}/{{.Secret}}"

type SsmManager struct {
	AwsAccessKeyID     string `long:"aws-access-key" description:"AWS Access key ID"`
	AwsSecretAccessKey string `long:"aws-secret-key" description:"AWS Secret Access Key"`
	AwsSessionToken    string `long:"aws-session-token" description:"AWS Session Token"`
	AwsRegion          string `long:"aws-region" description:"AWS region to send requests to. Enviroment variable AWS_REGION is used if this flag is not provided."`
	SecretTemplate     string `long:"secret-template" description:"AWS SSM parameter name template" default:"concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"`
	FallbackTemplate   string `long:"fallback-secret-template" description:"Fallback template to use for AWS SSM parameter name. Empty fallback template will disable this functionality" default:"concourse/{{.Team}}/{{.Secret}}"`
}

type SsmSecret struct {
	Team     string
	Pipeline string
	Secret   string
}

func (manager SsmManager) buildSecretTemplate() (*template.Template, error) {
	t, err := template.
		New("ssm-secret-name").
		Option("missingkey=error").
		Parse(manager.SecretTemplate)
	if err != nil {
		return nil, err
	}
	if parse.IsEmptyTree(t.Root) {
		return nil, errors.New("secret template should not be empty")
	}
	return t, nil
}

func (manager SsmManager) buildFallbackTemplate() (*template.Template, error) {
	t, err := template.
		New("ssm-secret-fallback-name").
		Option("missingkey=error").
		Parse(manager.FallbackTemplate)
	if err != nil {
		return nil, err
	}
	if parse.IsEmptyTree(t.Root) {
		return nil, nil
	}
	return t, nil
}

func (manager SsmManager) IsConfigured() bool {
	return manager.AwsRegion != "" || os.Getenv("AWS_REGION") != ""
}

func (manager SsmManager) Validate() error {
	// Make sure that the template is valid
	secretTemplate, err := manager.buildSecretTemplate()
	if err != nil {
		return err
	}
	fallbackTemplate, err := manager.buildFallbackTemplate()
	if err != nil {
		return err
	}
	// Execute the templates on dummy data to verify that it does not expect additional data
	dummy := SsmSecret{Team: "team", Pipeline: "pipeline", Secret: "secret"}
	err = secretTemplate.Execute(ioutil.Discard, &dummy)
	if err != nil {
		return err
	}
	if fallbackTemplate != nil {
		err = fallbackTemplate.Execute(ioutil.Discard, &dummy)
		if err != nil {
			return err
		}
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

func (manager SsmManager) NewVariablesFactory(lager.Logger) (creds.VariablesFactory, error) {
	config := &aws.Config{Region: &manager.AwsRegion}
	if manager.AwsAccessKeyID != "" {
		config.Credentials = credentials.NewStaticCredentials(manager.AwsAccessKeyID, manager.AwsSecretAccessKey, manager.AwsSessionToken)
	}

	session, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}

	secretTemplate, err := manager.buildSecretTemplate()
	if err != nil {
		return nil, err
	}

	fallbackTemplate, err := manager.buildFallbackTemplate()
	if err != nil {
		return nil, err
	}

	return NewSsmFactory(session, secretTemplate, fallbackTemplate), nil
}
