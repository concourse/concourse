package secretsmanager

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/concourse/concourse/atc/creds"
)

const managerName = "secretsmanager"

const DefaultPipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultTeamSecretTemplate = "/concourse/{{.Team}}/{{.Secret}}"

type Manager struct {
	Enabled                bool   `yaml:"enabled,omitempty"`
	AwsAccessKeyID         string `yaml:"access_key,omitempty"`
	AwsSecretAccessKey     string `yaml:"secret_key,omitempty"`
	AwsSessionToken        string `yaml:"session_token,omitempty"`
	AwsRegion              string `yaml:"region,omitempty"`
	PipelineSecretTemplate string `yaml:"pipeline_secret_template,omitempty"`
	TeamSecretTemplate     string `yaml:"team_secret_template,omitempty"`
	SecretManager          *SecretsManager
}

func (manager *Manager) Name() string {
	return managerName
}

func (manager *Manager) Config() interface{} {
	return manager
}

func (manager *Manager) Init(log lager.Logger) error {
	config := &aws.Config{Region: &manager.AwsRegion}
	if manager.AwsAccessKeyID != "" {
		config.Credentials = credentials.NewStaticCredentials(manager.AwsAccessKeyID, manager.AwsSecretAccessKey, manager.AwsSessionToken)
	}

	sess, err := session.NewSession(config)
	if err != nil {
		log.Error("create-aws-session", err)
		return err
	}

	manager.SecretManager = &SecretsManager{
		log: log,
		api: secretsmanager.New(sess),
	}
	return nil
}

func (manager *Manager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "GetSecretValue",
	}

	_, _, _, err := manager.SecretManager.getSecretById("__concourse-health-check")
	if err != nil {
		health.Error = err.Error()
		return health, nil
	}

	health.Response = map[string]string{
		"status": "UP",
	}

	return health, nil
}

func (manager *Manager) MarshalJSON() ([]byte, error) {
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

func (manager *Manager) Validate() error {
	if manager.AwsRegion == "" {
		return errors.New("must provide aws region")
	}

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

func (manager *Manager) NewSecretsFactory(log lager.Logger) (creds.SecretsFactory, error) {
	config := &aws.Config{Region: &manager.AwsRegion}
	if manager.AwsAccessKeyID != "" {
		config.Credentials = credentials.NewStaticCredentials(manager.AwsAccessKeyID, manager.AwsSecretAccessKey, manager.AwsSessionToken)
	}

	sess, err := session.NewSession(config)
	if err != nil {
		log.Error("create-aws-session", err)
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

	return NewSecretsManagerFactory(log, sess, []*creds.SecretTemplate{pipelineSecretTemplate, teamSecretTemplate}), nil
}

func (manager Manager) Close(logger lager.Logger) {
	// TODO - to implement
}
