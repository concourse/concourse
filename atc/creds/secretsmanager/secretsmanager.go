package secretsmanager

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/concourse/concourse/atc/creds"

	lager "code.cloudfoundry.org/lager/v3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . SecretsManagerAPI
type SecretsManagerAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type SecretsManager struct {
	log             lager.Logger
	api             SecretsManagerAPI
	secretTemplates []*creds.SecretTemplate
}

func NewSecretsManager(log lager.Logger, api SecretsManagerAPI, secretTemplates []*creds.SecretTemplate) *SecretsManager {
	return &SecretsManager{
		log:             log,
		api:             api,
		secretTemplates: secretTemplates,
	}
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (s *SecretsManager) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, tmpl := range s.secretTemplates {
		if lPath := creds.NewSecretLookupWithTemplate(tmpl, teamName, pipelineName); lPath != nil {
			lookupPaths = append(lookupPaths, lPath)
		}
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (s *SecretsManager) Get(secretPath string) (any, *time.Time, bool, error) {
	value, expiration, found, err := s.getSecretById(secretPath)
	if err != nil {
		s.log.Error("failed-to-fetch-aws-secret", err, lager.Data{
			"secret-path": secretPath,
		})
		return nil, nil, false, err
	}
	if found {
		return value, expiration, true, nil
	}
	return nil, nil, false, nil
}

/*
Looks up secret by path. Depending on which field is filled it will either
return a string value (SecretString) or a map[string]any (SecretBinary).

In case SecretBinary is set, it is expected to be a valid JSON object or it will error.
*/
func (s *SecretsManager) getSecretById(path string) (any, *time.Time, bool, error) {
	ctx := context.TODO()
	value, err := s.api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &path,
	})
	if err == nil {
		switch {
		case value.SecretString != nil:
			values, err := decodeJsonValue([]byte(*value.SecretString))
			if err != nil {
				return *value.SecretString, nil, true, nil
			}
			return values, nil, true, nil
		case value.SecretBinary != nil:
			values, err := decodeJsonValue(value.SecretBinary)
			if err != nil {
				return nil, nil, true, err
			}
			return values, nil, true, nil
		}
	} else {
		var notFound *types.ResourceNotFoundException
		// a secret that's marked for deletion will return an invalid request exception as an error
		var invalidRequest *types.InvalidRequestException
		if errors.As(err, &notFound) || errors.As(err, &invalidRequest) {
			return nil, nil, false, nil
		}
	}

	return nil, nil, false, err
}

func decodeJsonValue(data []byte) (map[string]any, error) {
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	return values, nil
}
