package ssm

import (
	"context"
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/lager/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go"
	"github.com/concourse/concourse/atc/creds"
)

const (
	DefaultPipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
	DefaultTeamSecretTemplate     = "/concourse/{{.Team}}/{{.Secret}}"
)

type SsmManager struct {
	AwsAccessKeyID         string `mapstructure:"access_key" long:"access-key" description:"AWS Access key ID"`
	AwsSecretAccessKey     string `mapstructure:"secret_key" long:"secret-key" description:"AWS Secret Access Key"`
	AwsSessionToken        string `mapstructure:"session_token" long:"session-token" description:"AWS Session Token"`
	AwsRegion              string `mapstructure:"region" long:"region" description:"AWS region to send requests to"`
	PipelineSecretTemplate string `mapstructure:"pipeline_secret_template" long:"pipeline-secret-template" description:"AWS SSM parameter name template used for pipeline specific parameter" default:"/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"`
	TeamSecretTemplate     string `mapstructure:"team_secret_template" long:"team-secret-template" description:"AWS SSM parameter name template used for team specific parameter" default:"/concourse/{{.Team}}/{{.Secret}}"`
	SharedPath             string `mapstructure:"shared_path" long:"shared-path" description:"AWS SSM parameter path used for shared parameters"`
	Ssm                    *Ssm
}

func (manager *SsmManager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]any{
		"aws_region":               manager.AwsRegion,
		"pipeline_secret_template": manager.PipelineSecretTemplate,
		"team_secret_template":     manager.TeamSecretTemplate,
		"shared_path":              manager.SharedPath,
		"health":                   health,
	})
}

func (manager *SsmManager) Init(log lager.Logger) error {
	cfg, err := manager.awsConfig()
	if err != nil {
		log.Error("failed-to-create-aws-config", err)
		return err
	}

	manager.Ssm = &Ssm{
		api: ssm.NewFromConfig(cfg),
	}

	return nil
}

func (manager *SsmManager) awsConfig() (aws.Config, error) {
	ctx := context.TODO()

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(manager.AwsRegion),
	}

	if manager.AwsAccessKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			manager.AwsAccessKeyID, manager.AwsSecretAccessKey, manager.AwsSessionToken,
		)))
	}

	return config.LoadDefaultConfig(ctx, opts...)
}

func (manager *SsmManager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "GetParameter",
	}

	_, _, _, err := manager.Ssm.getParameterByName("__concourse-health-check")
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) && apiError.ErrorCode() == "AccessDeniedException" {
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
	cfg, err := manager.awsConfig()
	if err != nil {
		log.Error("failed-to-create-aws-config", err)
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

	return NewSsmFactory(log, cfg, []*creds.SecretTemplate{pipelineSecretTemplate, teamSecretTemplate}, manager.SharedPath), nil
}

func (manager *SsmManager) Close(logger lager.Logger) {
	// TODO - to implement
}
