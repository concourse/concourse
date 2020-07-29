package secretsmanager

import (
	"encoding/json"
	"time"

	"github.com/concourse/concourse/atc/creds"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
)

type SecretsManager struct {
	log             lager.Logger
	api             secretsmanageriface.SecretsManagerAPI
	secretTemplates []*creds.SecretTemplate
}

func NewSecretsManager(log lager.Logger, api secretsmanageriface.SecretsManagerAPI, secretTemplates []*creds.SecretTemplate) *SecretsManager {
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
func (s *SecretsManager) Get(secretPath string) (interface{}, *time.Time, bool, error) {
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
	return a string value (SecretString) or a map[string]interface{} (SecretBinary).

	In case SecretBinary is set, it is expected to be a valid JSON object or it will error.
*/
func (s *SecretsManager) getSecretById(path string) (interface{}, *time.Time, bool, error) {
	value, err := s.api.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: &path,
	})
	if err == nil {
		switch {
		case value.SecretString != nil:
			return *value.SecretString, nil, true, nil
		case value.SecretBinary != nil:
			values, err := decodeJsonValue(value.SecretBinary)
			if err != nil {
				return nil, nil, true, err
			}
			return values, nil, true, nil
		}
	} else if errObj, ok := err.(awserr.Error); ok && errObj.Code() == secretsmanager.ErrCodeResourceNotFoundException {
		return nil, nil, false, nil
	}

	return nil, nil, false, err
}

func decodeJsonValue(data []byte) (map[string]interface{}, error) {
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	return values, nil
}
