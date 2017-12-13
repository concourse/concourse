package ssm

import (
	"bytes"
	"strings"
	"text/template"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"
)

type Ssm struct {
	log              lager.Logger
	api              ssmiface.SSMAPI
	TeamName         string
	PipelineName     string
	SecretTemplate   *template.Template
	FallbackTemplate *template.Template
}

func NewSsm(log lager.Logger, api ssmiface.SSMAPI, teamName string, pipelineName string, secretTemplate *template.Template, fallbackTemplate *template.Template) *Ssm {
	return &Ssm{
		log:              log,
		api:              api,
		TeamName:         teamName,
		PipelineName:     pipelineName,
		SecretTemplate:   secretTemplate,
		FallbackTemplate: fallbackTemplate,
	}
}

func (s *Ssm) buildSecretName(nameTemplate *template.Template, varName string) (string, error) {
	var buf bytes.Buffer
	err := nameTemplate.Execute(&buf, &SsmSecret{
		Team:     s.TeamName,
		Pipeline: s.PipelineName,
		Secret:   varName,
	})
	return buf.String(), err
}

func (s *Ssm) Get(varDef varTemplate.VariableDefinition) (interface{}, bool, error) {
	value, found, err := s.GetVar(s.SecretTemplate, varDef.Name)
	if !found && s.FallbackTemplate != nil {
		value, found, err = s.GetVar(s.FallbackTemplate, varDef.Name)
	}
	return value, found, err
}

func (s *Ssm) GetVar(nameTemplate *template.Template, varName string) (interface{}, bool, error) {
	secretName, err := s.buildSecretName(nameTemplate, varName)
	if err != nil {
		s.log.Error("Failed to build variable path from secret name", err, lager.Data{
			"template": nameTemplate.Name(),
			"secret":   varName,
		})
		return nil, false, err
	}
	s.log.Info("Trying to get SSM parameter by name", lager.Data{"name": secretName})
	// Try to get parameter as a string value
	param, err := s.api.GetParameter(&ssm.GetParameterInput{
		Name:           &secretName,
		WithDecryption: aws.Bool(true),
	})
	if err == nil {
		return *param.Parameter.Value, true, nil

	} else if errObj, ok := err.(awserr.Error); !ok || errObj.Code() != ssm.ErrCodeParameterNotFound {
		s.log.Error("Failed to fetch parameter from SSM", err, lager.Data{"parameter": secretName})
		return nil, false, err
	}
	// The parameter may exist as a complex object. So try to find all parameters in the path
	// this will be retuned as a map
	secretPath := strings.TrimRight(secretName, "/")
	if secretPath == "" {
		secretPath = "/"
	}
	s.log.Info("Trying to get SSM parameter by path", lager.Data{"path": secretPath})
	value := make(map[interface{}]interface{})
	pathQuery := &ssm.GetParametersByPathInput{}
	pathQuery = pathQuery.SetPath(secretPath).SetRecursive(true).SetWithDecryption(true).SetMaxResults(10)
	err = s.api.GetParametersByPathPages(pathQuery, func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
		for _, param := range page.Parameters {
			value[(*param.Name)[len(secretPath)+1:]] = *param.Value
		}
		return true
	})
	if err != nil {
		return nil, false, err
	}
	if len(value) == 0 {
		s.log.Info("SSM secret does not exists", lager.Data{"name": varName})
		return nil, false, nil
	}
	return value, true, nil
}

func (s *Ssm) List() ([]varTemplate.VariableDefinition, error) {
	// not implemented, see vault implementation
	return []varTemplate.VariableDefinition{}, nil
}
