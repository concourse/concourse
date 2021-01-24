package ssm

import (
	"encoding/json"
	"errors"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/concourse/concourse/atc/creds"
)

const DefaultPipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultTeamSecretTemplate = "/concourse/{{.Team}}/{{.Secret}}"

type SsmManager struct {
	AwsAccessKeyID         string `yaml:"access_key"`
	AwsSecretAccessKey     string `yaml:"secret_key"`
	AwsSessionToken        string `yaml:"session_token"`
	AwsRegion              string `yaml:"region"`
	PipelineSecretTemplate string `yaml:"pipeline_secret_template"`
	TeamSecretTemplate     string `yaml:"team_secret_template"`
	Ssm                    *Ssm
}

func (manager *SsmManager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]interface{}{
		"aws_region":               manager.AwsRegion,
		"pipeline_secret_template": manager.PipelineSecretTemplate,
		"team_secret_template":     manager.TeamSecretTemplate,
		"health":                   health,
	})
}

func (manager *SsmManager) Init(log lager.Logger) error {
	session, err := manager.getSession()
	if err != nil {
		log.Error("failed-to-create-aws-session", err)
		return err
	}

	manager.Ssm = &Ssm{
		api: ssm.New(session),
	}

	return nil
}

func (manager *SsmManager) getSession() (*session.Session, error) {

	config := &aws.Config{Region: &manager.AwsRegion}
	if manager.AwsAccessKeyID != "" {
		config.Credentials = credentials.NewStaticCredentials(manager.AwsAccessKeyID, manager.AwsSecretAccessKey, manager.AwsSessionToken)
	}

	return session.NewSession(config)
}

func (manager *SsmManager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "GetParameter",
	}

	_, _, _, err := manager.Ssm.getParameterByName("__concourse-health-check")
	if err != nil {
		if errObj, ok := err.(awserr.Error); ok && strings.Contains(errObj.Code(), "AccessDenied") {
			health.Response = map[string]string{
				"status": "UP",
			}

			return health, nil
		}

		health.Error = err.Error()
		return health, nil
	}

	health.Response = map[string]string{
		"status": "UP",
	}

	return health, nil
}

func (manager *SsmManager) IsConfigured() bool {
	return manager.AwsRegion != ""
}

func (manager *SsmManager) Validate() error {
	if _, err := creds.BuildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate); err != nil {
		return err
	}

	if _, err := creds.BuildSecretTemplate("team-secret-template", manager.TeamSecretTemplate); err != nil {
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

func (manager *SsmManager) NewSecretsFactory(log lager.Logger) (creds.SecretsFactory, error) {

	session, err := manager.getSession()
	if err != nil {
		log.Error("failed-to-create-aws-session", err)
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

	return NewSsmFactory(log, session, []*creds.SecretTemplate{pipelineSecretTemplate, teamSecretTemplate}), nil
}

func (manager *SsmManager) Close(logger lager.Logger) {
	// TODO - to implement
}
