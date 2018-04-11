package secretsmanager

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"

	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"
)

type SecretsManager struct {
	log             lager.Logger
	api             secretsmanageriface.SecretsManagerAPI
	TeamName        string
	PipelineName    string
	SecretTemplates []*template.Template
}

func NewSecretsManager(log lager.Logger, api secretsmanageriface.SecretsManagerAPI, teamName string, pipelineName string, secretTemplates []*template.Template) *SecretsManager {
	return &SecretsManager{
		log:             log,
		api:             api,
		TeamName:        teamName,
		PipelineName:    pipelineName,
		SecretTemplates: secretTemplates,
	}
}

func (s *SecretsManager) buildSecretId(nameTemplate *template.Template, secret string) (string, error) {
	var buf bytes.Buffer
	err := nameTemplate.Execute(&buf, &Secret{
		Team:     s.TeamName,
		Pipeline: s.PipelineName,
		Secret:   secret,
	})
	return buf.String(), err
}

func (s *SecretsManager) Get(varDef varTemplate.VariableDefinition) (interface{}, bool, error) {
	for _, st := range s.SecretTemplates {
		secretId, err := s.buildSecretId(st, varDef.Name)
		if err != nil {
			s.log.Error("build-secret-id", err, lager.Data{"template": st.Name(), "secret": varDef.Name})
			return nil, false, err
		}

		if strings.Contains(secretId, "//") {
			continue
		}

		value, found, err := s.getSecretById(secretId)
		if err != nil {
			s.log.Error("get-secret", err, lager.Data{
				"template": st.Name(), "secret": varDef.Name, "secretId": secretId,
			})
			return nil, false, err
		}
		if found {
			return value, true, nil
		}
	}
	return nil, false, nil
}

/*
	Looks up secret by name. Depending on which field is filled it will either
	return a string value (SecretString) or a map[interface{}]interface{} (SecretBinary).

	In case SecretBinary is set, it is expected to be a valid JSON object or it will error.
*/
func (s *SecretsManager) getSecretById(name string) (interface{}, bool, error) {
	value, err := s.api.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: &name,
	})
	if err == nil {
		switch {
		case value.SecretString != nil:
			return *value.SecretString, true, nil
		case value.SecretBinary != nil:
			values, err := decodeJsonValue(value.SecretBinary)
			if err != nil {
				return nil, true, err
			}
			return values, true, nil
		}
	} else if errObj, ok := err.(awserr.Error); ok && errObj.Code() == secretsmanager.ErrCodeResourceNotFoundException {
		return nil, false, nil
	}

	return nil, false, err
}

func (s *SecretsManager) List() ([]varTemplate.VariableDefinition, error) {
	// not implemented, see vault implementation
	return []varTemplate.VariableDefinition{}, nil
}

func decodeJsonValue(data []byte) (map[interface{}]interface{}, error) {
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	evenLessTyped := map[interface{}]interface{}{}
	for k, v := range values {
		evenLessTyped[k] = v
	}
	return evenLessTyped, nil
}
